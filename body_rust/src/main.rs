// Author: Enkae (enkae.dev@pm.me)
mod accessibility;
mod effector;

use accessibility::UIElement;
use anyhow::Result;
use std::time::Duration;
use tokio::sync::mpsc;
use tokio::time;
use windows::Win32::System::Com::{
    CoCreateInstance, CoInitializeEx, CLSCTX_INPROC_SERVER, COINIT_MULTITHREADED,
};
use windows::Win32::UI::Accessibility::{CUIAutomation, IUIAutomation};

// Import generated proto code
pub mod ghost_proto {
    tonic::include_proto!("ghost");
}
use ghost_proto::nervous_system_client::NervousSystemClient;
use ghost_proto::FocusState;

#[tokio::main]
async fn main() -> Result<()> {
    println!("[SENTINEL] Body Online. Connecting to Conscience...");

    // 1. Connect to Kernel (gRPC)
    let client = loop {
        match NervousSystemClient::connect("http://127.0.0.1:50051").await {
            Ok(c) => {
                println!("[SENTINEL] Connected to Nervous System.");
                break c;
            }
            Err(_) => {
                println!("[SENTINEL] Waiting for Kernel...");
                time::sleep(Duration::from_secs(2)).await;
            }
        }
    };

    // Clone for the two concurrent tasks
    let mut focus_client = client.clone();
    let mut action_client = client;

    println!("[SENTINEL] Vision Active. Streaming Focus...");

    // === TASK 1: Focus reporting (existing) ===
    let (tx, mut rx) = mpsc::channel::<FocusState>(32);

    // COM/UI Automation must run on a dedicated thread
    std::thread::spawn(move || {
        unsafe { CoInitializeEx(None, COINIT_MULTITHREADED) }
            .ok()
            .expect("COM initialization failed");

        let automation: IUIAutomation = unsafe {
            CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER)
                .expect("Failed to create UIAutomation")
        };

        loop {
            std::thread::sleep(Duration::from_millis(500));

            if let Ok(element) = capture_focused_element(&automation) {
                let focus = FocusState {
                    window_title: element.name.clone(),
                    process_name: "Unknown".to_string(),
                    ui_tree_snapshot: "".to_string(),
                };

                if tx.blocking_send(focus).is_err() {
                    break;
                }
            }
        }
    });

    let focus_stream = async_stream::stream! {
        while let Some(focus) = rx.recv().await {
            yield focus;
        }
    };

    // Spawn focus reporting as background task
    let focus_handle = tokio::spawn(async move {
        let request = tonic::Request::new(focus_stream);
        match focus_client.report_focus(request).await {
            Ok(_) => println!("[SENTINEL] Focus stream completed."),
            Err(e) => println!("[SENTINEL] Focus stream error: {}", e),
        }
    });

    // === TASK 2: Receive approved actions via StreamActions ===
    let action_handle = tokio::spawn(async move {
        println!("[SENTINEL] Subscribing to Action Stream...");

        let request = tonic::Request::new(());
        match action_client.stream_actions(request).await {
            Ok(response) => {
                let mut stream = response.into_inner();

                // Spawn a dedicated thread for the Effector (needs its own thread for input sim)
                let (action_tx, action_rx) =
                    std::sync::mpsc::channel::<ghost_proto::ActionCommand>();

                std::thread::spawn(move || {
                    let mut eff = match effector::Effector::new() {
                        Ok(e) => {
                            println!("[EFFECTOR] Initialized.");
                            e
                        }
                        Err(err) => {
                            eprintln!("[EFFECTOR] Failed to initialize: {}", err);
                            return;
                        }
                    };

                    while let Ok(cmd) = action_rx.recv() {
                        if let Some(action) = &cmd.action {
                            let action_type = action.r#type.to_uppercase();
                            println!(
                                "[EFFECTOR] Executing: {} ({})",
                                action_type, cmd.command_id
                            );

                            // Convert proto payload map<string,string> to serde_json::Value
                            let result = match action_type.as_str() {
                                "TYPE" => {
                                    let text = action
                                        .payload
                                        .get("text")
                                        .map(|s| s.as_str())
                                        .unwrap_or("");
                                    let payload = serde_json::json!({"text": text});
                                    eff.execute_type_text(&payload)
                                }
                                "KEY" => {
                                    let key = action
                                        .payload
                                        .get("key")
                                        .map(|s| s.as_str())
                                        .unwrap_or("");
                                    let payload = serde_json::json!({"key": key});
                                    eff.execute_press_key(&payload)
                                }
                                "CLICK" => {
                                    let x: i64 = action
                                        .payload
                                        .get("x")
                                        .and_then(|s| s.parse().ok())
                                        .unwrap_or(0);
                                    let y: i64 = action
                                        .payload
                                        .get("y")
                                        .and_then(|s| s.parse().ok())
                                        .unwrap_or(0);
                                    let payload = serde_json::json!({"x": x, "y": y});
                                    eff.execute_click(&payload)
                                }
                                _ => {
                                    println!("[EFFECTOR] Unknown action type: {}", action_type);
                                    Ok(())
                                }
                            };

                            match result {
                                Ok(()) => {
                                    println!("[EFFECTOR] Completed: {}", cmd.command_id)
                                }
                                Err(e) => {
                                    eprintln!("[EFFECTOR] Failed {}: {}", cmd.command_id, e)
                                }
                            }
                        }
                    }
                });

                // Read actions from gRPC stream and forward to effector thread
                while let Ok(Some(cmd)) = stream.message().await {
                    println!("[SENTINEL] Action received: {}", cmd.command_id);
                    let _ = action_tx.send(cmd);
                }
            }
            Err(e) => {
                eprintln!("[SENTINEL] Failed to connect to action stream: {}", e);
            }
        }
    });

    // Wait for either task to end
    tokio::select! {
        _ = focus_handle => println!("[SENTINEL] Focus task ended."),
        _ = action_handle => println!("[SENTINEL] Action task ended."),
    }

    Ok(())
}

// Helper to get focus
fn capture_focused_element(automation: &IUIAutomation) -> Result<UIElement> {
    let element = unsafe { automation.GetFocusedElement()? };
    let name = unsafe {
        element
            .CurrentName()
            .map(|s| s.to_string())
            .unwrap_or_else(|_| String::from("Unknown"))
    };
    Ok(UIElement {
        name,
        control_type: "Window".to_string(),
        bounding_rectangle: "".to_string(),
        children: vec![],
    })
}
