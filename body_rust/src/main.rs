// Author: Enkae (enkae.dev@pm.me)
mod accessibility;

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
    let mut client = loop {
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

    println!("[SENTINEL] Vision Active. Streaming Focus...");

    // 2. Create a channel for focus updates
    let (tx, mut rx) = mpsc::channel::<FocusState>(32);

    // 3. Spawn a blocking thread for COM operations
    // COM/UI Automation must run on a dedicated thread
    std::thread::spawn(move || {
        // Initialize COM on this thread
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

                // Send to async runtime; break if receiver is dropped
                if tx.blocking_send(focus).is_err() {
                    break;
                }
            }
        }
    });

    // 4. Create a stream from the channel receiver (this is Send-safe)
    let stream = async_stream::stream! {
        while let Some(focus) = rx.recv().await {
            yield focus;
        }
    };

    // 5. Send the stream to the Kernel (single persistent connection)
    let request = tonic::Request::new(stream);
    match client.report_focus(request).await {
        Ok(_) => println!("[SENTINEL] Focus stream completed."),
        Err(e) => println!("[SENTINEL] Focus stream error: {}", e),
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
