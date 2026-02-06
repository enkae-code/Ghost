// Author: Enkae (enkae.dev@pm.me)
mod accessibility;

use accessibility::UIElement;
use anyhow::Result;
use std::time::Duration;
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
    println!("[SENTINEL] 🛡️ Body Online. Connecting to Conscience...");

    // 1. Connect to Kernel (gRPC)
    // Retry loop until connection is established
    let mut client = loop {
        match NervousSystemClient::connect("http://127.0.0.1:50051").await {
            Ok(c) => {
                println!("[SENTINEL] 🔗 Connected to Nervous System.");
                break c;
            }
            Err(_) => {
                println!("[SENTINEL] ⏳ Waiting for Kernel...");
                time::sleep(Duration::from_secs(2)).await;
            }
        }
    };

    // 2. Initialize Windows Accessibility (COM)
    unsafe { CoInitializeEx(None, COINIT_MULTITHREADED) }
        .ok()
        .ok_or_else(|| anyhow::anyhow!("COM initialization failed"))?;
    let automation: IUIAutomation =
        unsafe { CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER)? };

    println!("[SENTINEL] 👁️ Vision Active. Streaming Focus...");

    // 3. Focus Loop
    // We poll the focus every 500ms and stream it to the Kernel
    let mut interval = time::interval(Duration::from_millis(500));

    loop {
        interval.tick().await;

        if let Ok(element) = capture_focused_element(&automation) {
            let request = tonic::Request::new(async_stream::stream! {
                // We yield ONE item then break, effectively sending a "pulse"
                // Real implementation should keep the stream open.
                yield FocusState {
                    window_title: element.name.clone(),
                    process_name: "Unknown".to_string(), // TODO: Get process ID
                    ui_tree_snapshot: "".to_string(),    // Optimization: Don't send tree every frame
                };
            });

            // Note: In a proper stream, we wouldn't re-call ReportFocus every loop.
            // We would hold the stream open. For stability in V1, we just fire and forget.
            let _ = client.report_focus(request).await;
        }
    }
}

// Helper to get focus (Legacy Logic reused)
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
