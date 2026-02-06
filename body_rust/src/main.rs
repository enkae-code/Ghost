// Author: Enkae (enkae.dev@pm.me)
mod accessibility;
mod effector;
mod librarian;

use anyhow::Result;
use accessibility::UIElement;
use serde_json;
use std::fs;
use std::io::{self, BufRead, Write};
use std::net::TcpStream;
use std::path::PathBuf;
use std::sync::OnceLock;
use std::thread;
use std::time::Duration;
use windows::Win32::System::Com::{CoCreateInstance, CoInitializeEx, COINIT_MULTITHREADED, CLSCTX_INPROC_SERVER};
use windows::Win32::UI::Accessibility::{IUIAutomation, CUIAutomation};

const API_BASE_URL: &str = "http://localhost:3000";

static AUTH_TOKEN: OnceLock<Option<String>> = OnceLock::new();

#[derive(Debug, Clone, PartialEq)]
enum AppState {
    Active,
    Shadow,
    Paused,
}

impl AppState {
    fn from_str(s: &str) -> Option<Self> {
        match s {
            "ACTIVE" => Some(AppState::Active),
            "SHADOW" => Some(AppState::Shadow),
            "PAUSED" => Some(AppState::Paused),
            _ => None,
        }
    }
}

fn fetch_state() -> AppState {
    match reqwest::blocking::get(format!("{}/api/state", API_BASE_URL)) {
        Ok(response) => {
            if let Ok(json) = response.json::<serde_json::Value>() {
                if let Some(state_str) = json.get("state").and_then(|v| v.as_str()) {
                    return AppState::from_str(state_str).unwrap_or(AppState::Shadow);
                }
            }
            AppState::Shadow // Default to safe mode on error
        }
        Err(_) => AppState::Shadow, // Default to safe mode on error
    }
}

fn main() {
    if let Err(e) = real_main() {
        eprintln!("[SENTINEL] Fatal error: {}", e);
        std::process::exit(1);
    }
}

fn real_main() -> Result<()> {
    println!("[SENTINEL] Ghost Sentinel starting...");

    // Preload auth token so we fail fast if it's missing
    if get_auth_token().is_none() {
        eprintln!("[SENTINEL] ⚠️ ghost.token not found. Focus updates will be disabled.");
    }

    // Check if running in effector mode
    let args: Vec<String> = std::env::args().collect();
    let mode = args.get(1).map(|s| s.as_str());

    match mode {
        Some("--effector") => {
            println!("[SENTINEL] Running in EFFECTOR mode (action execution)");
            effector::effector_loop(API_BASE_URL);
            Ok(())
        }
        Some("--librarian") => {
            println!("[SENTINEL] Running in LIBRARIAN mode (file indexing)");
            run_librarian_mode()
        }
        Some("--capture") | None => {
            println!("[SENTINEL] Running in CAPTURE mode (accessibility tree)");
            run_capture_mode()
        }
        Some("--hybrid") => {
            println!("[SENTINEL] Running in HYBRID mode (IPC Daemon: stdin/stdout)");
            run_hybrid_daemon()
        }
        Some("--full") => {
            println!("[SENTINEL] Running in FULL mode (state-aware: capture + effector + librarian)");
            run_full_mode()
        }
        Some(unknown) => {
            eprintln!("[SENTINEL] Unknown mode: {}", unknown);
            eprintln!("Usage: engram-sentinel [--capture|--effector|--librarian|--hybrid|--full]");
            std::process::exit(1);
        }
    }
}

/// Hybrid mode: Long-lived daemon that reads JSON commands from stdin
fn run_hybrid_daemon() -> Result<()> {
    println!("[SENTINEL] Initializing IPC daemon...");

    // Initialize COM for stdin thread (needed for SCAN commands)
    // Note: Each thread that uses COM needs its own initialization
    let stdin_handle = thread::spawn(|| {
        // Initialize COM for this thread
        if unsafe { CoInitializeEx(None, COINIT_MULTITHREADED) }.is_err() {
            eprintln!("[SENTINEL] Failed to initialize COM in stdin thread");
            return;
        }

        // Create UI Automation instance for SCAN commands
        let scan_automation: Option<IUIAutomation> = unsafe {
            CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER).ok()
        };

        println!("[SENTINEL] Stdin listener active. Awaiting JSON commands...");
        let stdin = io::stdin();
        let reader = stdin.lock();

        for line in reader.lines() {
            match line {
                Ok(input_str) => {
                    let input_str = input_str.trim();
                    if input_str.is_empty() {
                        continue;
                    }

                    // Check for SCAN command (Sovereign Sight)
                    if input_str == "SCAN" {
                        if let Some(ref automation) = scan_automation {
                            match scan_full_tree(automation, 3) {
                                Ok(json_output) => {
                                    println!("[SCAN_RESULT]{}", json_output);
                                    if let Err(e) = io::stdout().flush() {
                                        eprintln!("[SENTINEL] Stdout flush error: {}", e);
                                    }
                                }
                                Err(e) => {
                                    eprintln!("[SENTINEL] Scan error: {}", e);
                                    println!("[SCAN_RESULT]{{\"error\": \"{}\"}}", e);
                                    let _ = io::stdout().flush();
                                }
                            }
                        } else {
                            eprintln!("[SENTINEL] SCAN unavailable: UI Automation not initialized");
                            println!("[SCAN_RESULT]{{\"error\": \"UI Automation not initialized\"}}");
                            let _ = io::stdout().flush();
                        }
                        continue;
                    }

                    // Execute the action (JSON command)
                    if let Err(e) = effector::execute_action_json(input_str) {
                        eprintln!("[SENTINEL] Action execution error: {}", e);
                    }
                }
                Err(e) => {
                    eprintln!("[SENTINEL] Stdin read error: {}", e);
                    break;
                }
            }
        }
    });

    // Initialize COM for the capture thread
    unsafe { CoInitializeEx(None, COINIT_MULTITHREADED) }.ok()?;

    let automation: IUIAutomation = unsafe {
        CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER)?
    };

    // Main thread: Periodic capture loop
    loop {
        thread::sleep(Duration::from_millis(500));

        // Capture focused element
        match capture_focused_element(&automation) {
            Ok(ui_element) => {
                // Send focus update to Go Kernel for safety verification
                notify_kernel_focus(&ui_element.name);

                // Output to Python via stdout
                if let Ok(json_output) = serde_json::to_string(&ui_element) {
                    println!("{}", json_output);
                    if let Err(e) = io::stdout().flush() {
                        eprintln!("[SENTINEL] Stdout flush error: {}", e);
                    }
                }
            }
            Err(e) => {
                eprintln!("[SENTINEL] Capture error: {}", e);
            }
        }
    }
}

/// Helper function to send focus update to the Go Kernel
fn notify_kernel_focus(window_name: &str) {
    // Attempt to connect to the Kernel and send focus update
    // If kernel is unavailable, silently ignore (graceful degradation)
    let auth_token = match get_auth_token() {
        Some(token) => token,
        None => return,
    };

    if let Ok(mut stream) = TcpStream::connect("127.0.0.1:5005") {
        let auth_payload = serde_json::json!({ "auth_token": auth_token });
        if let Ok(auth_str) = serde_json::to_string(&auth_payload) {
            let _ = stream.write_all(format!("{}\n", auth_str).as_bytes());
        }

        let focus_update = serde_json::json!({
            "type": "focus_update",
            "window_name": window_name
        });

        if let Ok(json_str) = serde_json::to_string(&focus_update) {
            let message = format!("{}\n", json_str);
            let _ = stream.write_all(message.as_bytes());
            let _ = stream.flush();
        }
    }
    // Silently ignore connection failures (Kernel may not be running)
}

/// Scan the entire UI tree from root (Sovereign Sight)
/// Returns JSON string of the full UI tree up to max_depth
fn scan_full_tree(automation: &IUIAutomation, max_depth: u32) -> Result<String> {
    // Get the root element (Desktop)
    let root_element = unsafe { automation.GetRootElement()? };

    // Walk the tree using the accessibility module
    let ui_tree = accessibility::walk_tree(&root_element, 0, max_depth)?;

    // Serialize to JSON
    let json_output = serde_json::to_string(&ui_tree)?;
    Ok(json_output)
}

/// Helper function to capture focused UI element
fn capture_focused_element(automation: &IUIAutomation) -> Result<UIElement> {
    let element = unsafe { automation.GetFocusedElement()? };

    let name = unsafe {
        element.CurrentName()
            .map(|s| s.to_string())
            .unwrap_or_else(|_| String::from("Unknown"))
    };

    let control_type = unsafe {
        element.CurrentLocalizedControlType()
            .map(|s| s.to_string())
            .unwrap_or_else(|_| String::from("Unknown"))
    };

    let bounding_rectangle = unsafe {
        element.CurrentBoundingRectangle()
            .map(|rect| format!(
                "left={},top={},right={},bottom={}",
                rect.left, rect.top, rect.right, rect.bottom
            ))
            .unwrap_or_else(|_| String::from("Unknown"))
    };

    Ok(UIElement {
        name,
        control_type,
        bounding_rectangle,
        children: Vec::new(),
    })
}

fn get_auth_token() -> Option<&'static str> {
    AUTH_TOKEN
        .get_or_init(load_auth_token)
        .as_deref()
}

fn load_auth_token() -> Option<String> {
    let mut candidates = Vec::new();
    candidates.push(PathBuf::from("ghost.token"));
    candidates.push(PathBuf::from("bin").join("ghost.token"));

    if let Ok(exe_path) = std::env::current_exe() {
        if let Some(bin_dir) = exe_path.parent() {
            candidates.push(bin_dir.join("ghost.token"));
            if let Some(root_dir) = bin_dir.parent() {
                candidates.push(root_dir.join("ghost.token"));
            }
        }
    }

    for path in candidates {
        if let Ok(contents) = fs::read_to_string(&path) {
            let token = contents.trim().to_string();
            if !token.is_empty() {
                println!(
                    "[SENTINEL] Loaded auth token from {}",
                    path.to_string_lossy()
                );
                return Some(token);
            }
        }
    }

    None
}

fn run_capture_mode() -> Result<()> {
    unsafe { CoInitializeEx(None, COINIT_MULTITHREADED) }.ok()?;

    let automation: IUIAutomation = unsafe {
        CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER)?
    };

    let ui_element = capture_focused_element(&automation)?;
    let json_output = serde_json::to_string(&ui_element)?;
    println!("{}", json_output);
    io::stdout().flush()?;

    Ok(())
}

fn run_librarian_mode() -> Result<()> {
    use librarian::Librarian;
    use std::env;
    use std::path::PathBuf;

    let artifact_url = format!("{}/api/artifacts", API_BASE_URL);
    let mut librarian = Librarian::new(artifact_url);

    // Default directories to watch (common user locations on Windows)
    let home_dir = env::var("USERPROFILE").unwrap_or_else(|_| "C:\\Users\\Default".to_string());

    let default_dirs = vec![
        PathBuf::from(format!("{}\\Documents", home_dir)),
        PathBuf::from(format!("{}\\Desktop", home_dir)),
        PathBuf::from(format!("{}\\Downloads", home_dir)),
        PathBuf::from(format!("{}\\Pictures", home_dir)),
        PathBuf::from(format!("{}\\AppData\\Roaming", home_dir)), // Game saves often here
    ];

    // Add each directory that exists
    for dir in default_dirs {
        if dir.exists() {
            librarian.watch_directory(dir)?;
        } else {
            println!("[LIBRARIAN] Skipping non-existent directory: {}", dir.display());
        }
    }

    println!("[LIBRARIAN] Initial indexing complete, starting file watcher...");

    // Start watching (this blocks forever)
    librarian.start_watching()?;

    Ok(())
}

fn run_full_mode() -> Result<()> {
    use std::sync::{Arc, Mutex};

    let current_state = Arc::new(Mutex::new(fetch_state()));
    let state_clone_effector = Arc::clone(&current_state);
    let state_clone_librarian = Arc::clone(&current_state);

    println!("[SENTINEL] Initial state: {:?}", *current_state.lock().unwrap());

    // Spawn state poller thread
    let state_poller = Arc::clone(&current_state);
    thread::spawn(move || {
        loop {
            thread::sleep(Duration::from_secs(1));
            let new_state = fetch_state();
            let mut current = state_poller.lock().unwrap();
            if *current != new_state {
                let emoji = match new_state {
                    AppState::Active => "🟢",
                    AppState::Shadow => "🟡",
                    AppState::Paused => "🔴",
                };
                println!("[STATE] {} Switched to: {:?}", emoji, new_state);
                *current = new_state;
            }
        }
    });

    // Spawn effector thread (respects state)
    thread::spawn(move || {
        loop {
            let state = state_clone_effector.lock().unwrap().clone();
            match state {
                AppState::Active => {
                    // Only execute actions when ACTIVE
                    effector::effector_loop(API_BASE_URL);
                }
                AppState::Shadow | AppState::Paused => {
                    // In SHADOW or PAUSED, do not execute actions
                    thread::sleep(Duration::from_millis(500));
                }
            }
        }
    });

    // Spawn librarian thread (respects state)
    thread::spawn(move || {
        loop {
            let state = state_clone_librarian.lock().unwrap().clone();
            match state {
                AppState::Active | AppState::Shadow => {
                    // Index files in ACTIVE or SHADOW mode
                    if let Err(e) = run_librarian_mode() {
                        eprintln!("[LIBRARIAN] Error: {}", e);
                    }
                }
                AppState::Paused => {
                    // In PAUSED, do nothing
                    thread::sleep(Duration::from_secs(1));
                }
            }
        }
    });

    // Main thread: Run capture (respects state)
    unsafe { CoInitializeEx(None, COINIT_MULTITHREADED) }.ok()?;
    let automation: IUIAutomation = unsafe {
        CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER)?
    };

    loop {
        let state = current_state.lock().unwrap().clone();
        match state {
            AppState::Active | AppState::Shadow => {
                // Capture screen in ACTIVE or SHADOW mode
                match capture_focused_element(&automation) {
                    Ok(ui_element) => {
                        // Send focus update to Go Kernel
                        notify_kernel_focus(&ui_element.name);
                    }
                    Err(e) => {
                        eprintln!("[SENTINEL] Capture error: {}", e);
                    }
                }
                thread::sleep(Duration::from_millis(100));
            }
            AppState::Paused => {
                // In PAUSED, sleep and do nothing
                thread::sleep(Duration::from_secs(1));
            }
        }
    }
}
