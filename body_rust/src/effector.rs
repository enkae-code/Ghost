// Author: Enkae (enkae.dev@pm.me)
use anyhow::{Result, Context};
use enigo::{Enigo, Mouse, Keyboard, Settings, Direction, Key, Button};
use serde::{Deserialize, Serialize};
use std::thread;
use std::time::Duration;

/// ActionProposal represents an approved action from the Permission Kernel
#[derive(Debug, Deserialize, Serialize, Clone)]
pub struct ActionProposal {
    pub id: String,
    pub intent: String,
    pub payload: serde_json::Value,
    pub domain: String,
    pub status: String,
}

/// ActionPayload contains the specific parameters for each action type
#[derive(Debug, Deserialize)]
#[serde(untagged)]
pub enum ActionPayload {
    TypeText { text: String },
    Click { x: i32, y: i32 },
    PressKey { key: String },
}

/// Effector executes physical actions on behalf of the agent
pub struct Effector {
    enigo: Enigo,
}

impl Effector {
    /// Creates a new Effector instance
    pub fn new() -> Result<Self> {
        let enigo = Enigo::new(&Settings::default())
            .context("Failed to initialize input controller")?;

        Ok(Self { enigo })
    }

    /// Executes an approved action
    pub fn execute_action(&mut self, action: &ActionProposal) -> Result<()> {
        println!("[EFFECTOR] ⚡ Executing: {} ({})", action.intent, &action.id[..8]);
        println!("[EFFECTOR]    Domain: {}", action.domain);

        // Extract delay from payload, default to 100ms
        let delay_ms = action.payload
            .get("delay_ms")
            .and_then(|v| v.as_u64())
            .unwrap_or(100) as u64;

        // Small delay before execution for safety
        thread::sleep(Duration::from_millis(delay_ms));

        // Parse the intent to determine action type
        let intent_upper = action.intent.to_uppercase();

        if intent_upper.contains("TYPE") || intent_upper.contains("TEXT") {
            self.execute_type_text(&action.payload)?;
        } else if intent_upper.contains("CLICK") {
            self.execute_click(&action.payload)?;
        } else if intent_upper.contains("KEY") || intent_upper.contains("PRESS") {
            self.execute_press_key(&action.payload)?;
        } else {
            println!("[EFFECTOR] ⚠️  Unknown action type: {}", action.intent);
            println!("[EFFECTOR]    Payload: {:?}", action.payload);
        }

        println!("[EFFECTOR] ✓ Completed: {}", &action.id[..8]);

        Ok(())
    }

    /// Types text into the focused window
    pub fn execute_type_text(&mut self, payload: &serde_json::Value) -> Result<()> {
        let text = payload
            .get("text")
            .and_then(|v| v.as_str())
            .context("Missing 'text' field in payload")?;

        // Sanitize input to prevent injection attacks
        let sanitized = sanitize_text(text);
        
        if sanitized != text {
            println!("[EFFECTOR]    ⚠️  Input sanitized (removed {} control characters)", text.len() - sanitized.len());
        }

        println!("[EFFECTOR]    Typing: \"{}\"", sanitized);

        self.enigo.text(&sanitized)
            .context("Failed to type text")?;

        Ok(())
    }

    /// Clicks the mouse at specified coordinates
    pub fn execute_click(&mut self, payload: &serde_json::Value) -> Result<()> {
        let x = payload
            .get("x")
            .and_then(|v| v.as_i64())
            .context("Missing 'x' coordinate in payload")? as i32;

        let y = payload
            .get("y")
            .and_then(|v| v.as_i64())
            .context("Missing 'y' coordinate in payload")? as i32;

        // Extract dynamic delay, default to 100ms
        let delay_ms = payload
            .get("delay_ms")
            .and_then(|v| v.as_u64())
            .unwrap_or(100) as u64;

        println!("[EFFECTOR]    Moving mouse to ({}, {})", x, y);

        // Move mouse to position
        self.enigo.move_mouse(x, y, enigo::Coordinate::Abs)
            .context("Failed to move mouse")?;

        // Configurable delay for visual feedback
        thread::sleep(Duration::from_millis(delay_ms));

        println!("[EFFECTOR]    Clicking left button");

        // Click
        self.enigo.button(Button::Left, Direction::Click)
            .context("Failed to click mouse")?;

        Ok(())
    }

    /// Presses a specific key (supports combo keys like "win+r", "ctrl+k")
    pub fn execute_press_key(&mut self, payload: &serde_json::Value) -> Result<()> {
        let key_str = payload
            .get("key")
            .and_then(|v| v.as_str())
            .context("Missing 'key' field in payload")?;

        println!("[EFFECTOR]    Pressing key: {}", key_str);

        // Handle combo keys like "win+r", "ctrl+k"
        if key_str.contains('+') {
            let parts: Vec<&str> = key_str.split('+').collect();
            let mut keys_to_press: Vec<Key> = Vec::new();

            for part in &parts {
                let k = match part.trim().to_uppercase().as_str() {
                    "WIN" | "GUI" | "META" | "WINDOWS" => Key::Meta,
                    "CTRL" | "CONTROL" => Key::Control,
                    "ALT" => Key::Alt,
                    "SHIFT" => Key::Shift,
                    s if s.len() == 1 => Key::Unicode(s.chars().next().unwrap()),
                    other => anyhow::bail!("Unknown combo key part: {}", other),
                };
                keys_to_press.push(k);
            }

            // Press all keys down
            for k in &keys_to_press {
                self.enigo.key(*k, Direction::Press)
                    .context("Failed to press combo key")?;
            }
            // Release in reverse order
            for k in keys_to_press.iter().rev() {
                self.enigo.key(*k, Direction::Release)
                    .context("Failed to release combo key")?;
            }

            return Ok(());
        }

        // Map string to Key enum
        let key = match key_str.to_uppercase().as_str() {
            "ENTER" | "RETURN" => Key::Return,
            "ESC" | "ESCAPE" => Key::Escape,
            "SPACE" => Key::Space,
            "TAB" => Key::Tab,
            "BACKSPACE" => Key::Backspace,
            "DELETE" => Key::Delete,
            "LEFT" => Key::LeftArrow,
            "RIGHT" => Key::RightArrow,
            "UP" => Key::UpArrow,
            "DOWN" => Key::DownArrow,
            "HOME" => Key::Home,
            "END" => Key::End,
            "PAGEUP" => Key::PageUp,
            "PAGEDOWN" => Key::PageDown,
            "GUI" | "META" | "WINDOWS" => Key::Meta,
            _ => {
                // Try to parse as a single character
                if key_str.len() == 1 {
                    Key::Unicode(key_str.chars().next().ok_or_else(|| anyhow::anyhow!("Empty key string"))?)
                } else {
                    anyhow::bail!("Unknown key: {}", key_str);
                }
            }
        };

        self.enigo.key(key, Direction::Click)
            .context("Failed to press key")?;

        // Intent Pacing: Wait after Enter/Return to allow target app to gain focus
        if matches!(key, Key::Return) {
            println!("[EFFECTOR]    Intent pacing: Waiting 800ms for OS to catch up...");
            thread::sleep(Duration::from_millis(800));
        }

        Ok(())
    }
}

/// Executes a single action from JSON stdin format
/// Expected format: {"action": "TYPE"|"KEY"|"CLICK", "payload": {...}}
pub fn execute_action_json(json_str: &str) -> Result<()> {
    let command: serde_json::Value = serde_json::from_str(json_str)
        .context("Failed to parse JSON command")?;

    let action_type = command
        .get("action")
        .and_then(|v| v.as_str())
        .context("Missing 'action' field")?;

    let payload = command
        .get("payload")
        .context("Missing 'payload' field")?;

    let mut effector = Effector::new()?;

    match action_type.to_uppercase().as_str() {
        "TYPE" => effector.execute_type_text(payload)?,
        "KEY" => effector.execute_press_key(payload)?,
        "CLICK" => effector.execute_click(payload)?,
        _ => anyhow::bail!("Unknown action type: {}", action_type),
    }

    Ok(())
}

/// Sanitizes text input by removing non-printable control characters
/// Protects against injection attacks and malformed input
fn sanitize_text(input: &str) -> String {
    input
        .chars()
        .filter(|c| {
            // Allow printable characters, spaces, tabs, and newlines
            // Block other control characters (0x00-0x1F except tab/newline, 0x7F-0x9F)
            match *c {
                '\t' | '\n' | '\r' => true,  // Allow common whitespace
                c if c.is_control() => false, // Block all other control chars
                _ => true,                    // Allow everything else
            }
        })
        .collect()
}

// NOTE: effector_loop (old HTTP polling approach) removed.
// Actions now arrive via gRPC StreamActions in main.rs.

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_sanitize_text_removes_null_bytes() {
        let input = "Hello\x00World";
        let result = sanitize_text(input);
        assert_eq!(result, "HelloWorld", "Null bytes should be removed");
    }

    #[test]
    fn test_sanitize_text_removes_escape_codes() {
        let input = "Hello\x1B[31mWorld\x1B[0m";
        let result = sanitize_text(input);
        assert_eq!(result, "Hello[31mWorld[0m", "Escape codes should be removed");
    }

    #[test]
    fn test_sanitize_text_preserves_whitespace() {
        let input = "Hello\tWorld\nNew Line\rCarriage";
        let result = sanitize_text(input);
        assert_eq!(result, input, "Tabs, newlines, and carriage returns should be preserved");
    }

    #[test]
    fn test_sanitize_text_preserves_normal_text() {
        let input = "Hello World! 123 @#$%";
        let result = sanitize_text(input);
        assert_eq!(result, input, "Normal printable characters should be preserved");
    }

    #[test]
    fn test_sanitize_text_handles_unicode() {
        let input = "Hello 世界 🌍 Émoji";
        let result = sanitize_text(input);
        assert_eq!(result, input, "Unicode characters should be preserved");
    }

    #[test]
    fn test_sanitize_text_removes_multiple_control_chars() {
        let input = "\x00\x01\x02Hello\x03\x04World\x05\x06";
        let result = sanitize_text(input);
        assert_eq!(result, "HelloWorld", "All control characters should be removed");
    }

    #[test]
    fn test_sanitize_text_empty_string() {
        let input = "";
        let result = sanitize_text(input);
        assert_eq!(result, "", "Empty string should remain empty");
    }

    #[test]
    fn test_sanitize_text_only_control_chars() {
        let input = "\x00\x01\x02\x03\x04";
        let result = sanitize_text(input);
        assert_eq!(result, "", "String with only control chars should become empty");
    }

    #[test]
    fn test_sanitize_text_bell_character() {
        let input = "Alert\x07Sound";
        let result = sanitize_text(input);
        assert_eq!(result, "AlertSound", "Bell character (0x07) should be removed");
    }

    #[test]
    fn test_sanitize_text_backspace() {
        let input = "Hello\x08World";
        let result = sanitize_text(input);
        assert_eq!(result, "HelloWorld", "Backspace character should be removed");
    }

    // Property-based tests: verify sanitize_text never panics
    #[test]
    fn test_sanitize_text_never_panics_on_random_input() {
        // Test with various random-like inputs
        let test_cases: Vec<String> = vec![
            format!("\u{0000}\u{FFFF}"),  // Null and max BMP
            "a".repeat(10000),              // Very long string
            "\x00".repeat(1000),            // Many control chars
            "🎉".repeat(500),               // Many emojis
            "\n\r\t".repeat(100),           // Many whitespace chars
        ];

        for input in test_cases.iter() {
            let result = std::panic::catch_unwind(|| sanitize_text(input));
            assert!(result.is_ok(), "sanitize_text should never panic on input: {:?}", input);
        }
    }

    #[test]
    fn test_sanitize_text_idempotent() {
        let input = "Hello\x00World\x1B";
        let first_pass = sanitize_text(input);
        let second_pass = sanitize_text(&first_pass);
        assert_eq!(first_pass, second_pass, "Sanitizing twice should produce same result");
    }

    #[test]
    fn test_sanitize_text_length_never_increases() {
        let inputs = vec![
            "Hello\x00World",
            "\x1B[31mRed\x1B[0m",
            "Normal text",
            "\x00\x01\x02",
        ];

        for input in inputs {
            let result = sanitize_text(input);
            assert!(
                result.len() <= input.len(),
                "Sanitized output should never be longer than input. Input: {:?}, Output: {:?}",
                input, result
            );
        }
    }

    #[test]
    fn test_sanitize_text_preserves_order() {
        let input = "A\x00B\x01C\x02D";
        let result = sanitize_text(input);
        assert_eq!(result, "ABCD", "Character order should be preserved after sanitization");
    }
}
