# // Author: Enkae (enkae.dev@pm.me)
import json
import logging
import os
import queue
import random
import re
import signal
import socket
import sys
import threading
import time
import uuid
from pathlib import Path
import subprocess
try:
    import winsound
except ImportError:
    winsound = None
from colorama import init, Fore
from bridge import SentinelBridge
from brain.planner import GhostPlanner
from gui.animations import TrayState
from gui.tray import TrayIcon
from ears.hearing import WhisperEngine
from ears.recorder import AudioRecorder

class PiperEngine:
    """
    Neural Text-to-Speech Engine using Piper (Local, Private, Fast).
    """
    def __init__(self, bin_path: str, model_path: str):
        self.bin_path = bin_path
        self.model_path = model_path

    def say(self, text: str):
        if not text: return
        
        # Clean text for command line safety
        text = text.replace('"', '').replace('\n', ' ')
        
        # Use a temporary file in the current directory
        # In production, use tempfile module, but for simple debugging keep it local
        outfile = "voice_out.wav"
        
        cmd = [
            self.bin_path,
            "--model", self.model_path,
            "--output_file", outfile
        ]
        
        try:
            # Run Piper subprocess
            # Pipe text to stdin, suppress stdout/stderr
            process = subprocess.Popen(
                cmd, 
                stdin=subprocess.PIPE, 
                stdout=subprocess.DEVNULL, 
                stderr=subprocess.DEVNULL
            )
            process.communicate(input=text.encode('utf-8'))
            
            # Play Asynchronously (Non-blocking)
            if os.path.exists(outfile) and winsound:
                winsound.PlaySound(outfile, winsound.SND_FILENAME | winsound.SND_ASYNC)
                
        except Exception as e:
            print(Fore.RED + f"[VOICE] Piper Error: {e}")

try:
    import keyboard
except ImportError:
    keyboard = None


init(autoreset=True)

class Ghost:
    """
    Ghost - Voice-Enabled Digital Proxy
    Tri-Brain Architecture: Python (Orchestrator) + Rust (Body) + Go (Conscience)

    Key Features:
    - Config-driven operation (config.json)
    - Structured logging to file + console
    - Graceful shutdown with threading.Event coordination
    - Transactional TCP sockets to Go Kernel (connect-request-close per call)
    - Visual verification loop: waits for expected window focus before acting
    - Trust Score integration from SQLite memory system
    - Push-to-Talk voice commands with local Whisper transcription
    - Intent sanitization and command validation
    """
    def __init__(self, tray_icon: TrayIcon | None = None, shutdown_event: threading.Event | None = None):
        self.running = True
        self.shutdown_event = shutdown_event or threading.Event()
        self.config = self._load_config()
        self.logger = self._setup_logging()
        self.tray_icon = tray_icon
        
        # Voice command components
        self.whisper_engine: WhisperEngine | None = None
        self.audio_recorder: AudioRecorder | None = None
        self.voice_enabled = False
        self.voice_paused = False  # Flag to prevent feedback loop during execution
        self.processing_lock = threading.Lock()  # Mutex to prevent race conditions (Echo Loop)

        # Text-to-Speech engine (Voice Box)
        self.silent_mode = False  # Toggle for voice output
        
        # Initialize Piper Neural TTS
        project_root = Path(__file__).parent.parent.resolve()
        piper_bin = project_root / "bin" / "piper" / "piper.exe"
        model_path = project_root / "bin" / "piper" / "en_US-amy-medium.onnx"
        
        self.piper = PiperEngine(str(piper_bin), str(model_path))
        self.logger.info(f"Piper TTS initialized at {piper_bin}")

        # Extract config values
        self.kernel_host = self.config["network"]["kernel_host"]
        self.kernel_port = self.config["network"]["kernel_port"]
        self.retry_limit = self.config["vision"]["retry_limit"]
        self.retry_delay = self.config["vision"]["retry_delay_ms"] / 1000.0
        self.action_pacing = self.config["delays"]["action_pacing_ms"]
        
        # Load or generate authentication token
        self.auth_token = self._load_or_generate_token()
        self.logger.info("Authentication token loaded")
        
        self.logger.info("Ghost initializing...")
        self.logger.info(f"Version: {self.config['system']['version']}")
        self.logger.info(f"Environment: {self.config['system']['environment']}")
        
        print(Fore.CYAN + "[GHOST] 🧠 Initializing Brain (Llama 3.1)...")
        ollama_url = self.config["network"].get("ollama_url", "http://localhost:11434/api/generate")
        self.brain = GhostPlanner(ollama_url=ollama_url)
        
        # --- NEW WARMUP BLOCK ---
        print(Fore.CYAN + "[GHOST] 🔥 Warming up Neural Pathways...")
        try:
            # Send a dummy request to force-load the model into VRAM immediately
            import requests
            requests.post(
                "http://localhost:11434/api/generate",
                json={"model": "llama3.1", "keep_alive": -1}
            )
            print(Fore.GREEN + "[GHOST] ✓ Brain Pre-Loaded & Ready.")
        except Exception:
            print(Fore.YELLOW + "[GHOST] ⚠️ Warmup failed (non-critical).")
        # ------------------------
        self.logger.info("Brain module loaded")
        print(Fore.GREEN + "[GHOST] ✓ Brain Online.")

        print(Fore.CYAN + f"[GHOST] Kernel configured at {self.kernel_host}:{self.kernel_port} (transactional mode)")

        print(Fore.CYAN + "[GHOST] 👁️  Connecting to Sentinel...")
        self.body = SentinelBridge()
        if self.body.wake(mode="hybrid"):
            self.logger.info("Sentinel connected (hybrid mode)")
            print(Fore.GREEN + "[GHOST] ✓ Vision & Hands Connected.")
        else:
            self.logger.error("Failed to connect to Sentinel")
            print(Fore.RED + "[GHOST] ✗ Failed to connect to Sentinel.")
        
        # Initialize voice command system
        self._init_voice_system()

        # Register global panic button
        self._register_panic_button()

    def _load_config(self) -> dict:
        """Load configuration from config.json with safe defaults."""
        # Use absolute path from project root to prevent issues with admin launch
        project_root = Path(__file__).parent.parent.resolve()
        config_path = project_root / "config.json"
        bin_config_path = project_root / "bin" / "config.json"
        
        target_path = config_path if config_path.exists() else bin_config_path
        
        # Safe defaults
        default_config = {
            "system": {"version": "3.0.0", "environment": "development", "log_level": "INFO", "log_file": "ghost.log"},
            "network": {"kernel_host": "localhost", "kernel_port": 5005},
            "vision": {"retry_limit": 50, "retry_delay_ms": 100},
            "security": {"safe_mode": True, "blocked_keywords": []},
            "delays": {"action_pacing_ms": 100, "enter_pacing_ms": 1200}
        }
        
        try:
            if target_path.exists():
                with open(target_path, 'r') as f:
                    config = json.load(f)
                    # Merge with defaults to handle missing keys
                    for section, values in default_config.items():
                        if section not in config:
                            config[section] = values
                        else:
                            for key, val in values.items():
                                if key not in config[section]:
                                    config[section][key] = val
                    return config
            else:
                print(Fore.YELLOW + f"[CONFIG] config.json not found at {target_path}, using defaults")
                return default_config
        except Exception as e:
            print(Fore.RED + f"[CONFIG] Failed to load config: {e}, using defaults")
            return default_config
    
    def _setup_logging(self) -> logging.Logger:
        """Setup structured logging to file and console."""
        log_level = getattr(logging, self.config["system"]["log_level"], logging.INFO)
        log_file = self.config["system"]["log_file"]
        
        logger = logging.getLogger("Ghost")
        logger.setLevel(log_level)
        
        # File handler (detailed)
        file_handler = logging.FileHandler(log_file)
        file_handler.setLevel(log_level)
        file_formatter = logging.Formatter(
            '%(asctime)s | %(levelname)-8s | %(name)s | %(message)s',
            datefmt='%Y-%m-%d %H:%M:%S'
        )
        file_handler.setFormatter(file_formatter)
        
        # Console handler (minimal)
        console_handler = logging.StreamHandler()
        console_handler.setLevel(logging.WARNING)  # Only warnings/errors to console
        console_formatter = logging.Formatter('%(levelname)s: %(message)s')
        console_handler.setFormatter(console_formatter)
        
        logger.addHandler(file_handler)
        logger.addHandler(console_handler)
        
        return logger
    
    def _load_or_generate_token(self) -> str:
        """Load or generate authentication token for Kernel communication."""
        # Use absolute path from project root to prevent issues with admin launch
        project_root = Path(__file__).parent.parent.resolve()
        token_file = project_root / "ghost.token"
        bin_token_file = project_root / "bin" / "ghost.token"

        target_token_file = token_file if token_file.exists() else bin_token_file
        
        try:
            if target_token_file.exists():
                token = target_token_file.read_text().strip()
                if len(token) == 64:  # 32 bytes = 64 hex chars
                    self.logger.debug("Loaded auth token from ghost.token")
                    return token
            
            # Generate new token (32 random bytes as hex)
            # Generate new token (32 random bytes as hex)
            import secrets
            token = secrets.token_hex(32)
            # Default to root if neither exists
            write_target = token_file 
            write_target.write_text(token)
            write_target.chmod(0o600)  # Restrict permissions
            self.logger.info("Generated new auth token: ghost.token")
            return token
        
        except Exception as e:
            self.logger.error(f"Failed to load/generate token: {e}")
            # Return a fallback token (not secure, but allows operation)
            return "0" * 64
    
    def _request_kernel(self, request: dict) -> dict | None:
        """
        Send a transactional request to the Go Kernel (connect-request-close).

        This eliminates socket thrashing by using short-lived connections.
        Each call opens a new socket, authenticates, sends request, receives response, and closes.

        Args:
            request: Dictionary to send as JSON

        Returns:
            dict: Parsed JSON response, or None on failure
        """
        sock = None
        try:
            # Create fresh socket for this transaction
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(2.0)  # Strict timeout for transactional calls
            sock.connect((self.kernel_host, self.kernel_port))

            # Authenticate first
            auth_msg = json.dumps({"auth_token": self.auth_token}) + "\n"
            sock.sendall(auth_msg.encode('utf-8'))

            # Send the actual request
            request_json = json.dumps(request) + "\n"
            sock.sendall(request_json.encode('utf-8'))

            # Receive response
            response_data = sock.recv(4096).decode('utf-8').strip()
            return json.loads(response_data)

        except (socket.timeout, ConnectionRefusedError, OSError) as e:
            self.logger.debug(f"Kernel unavailable: {e}")
            return None
        except json.JSONDecodeError as e:
            self.logger.warning(f"Kernel response parse error: {e}")
            return None
        finally:
            # Always close the socket
            if sock:
                try:
                    sock.close()
                except Exception:
                    pass
    
    def _invalidate_reflex(self, intent: str) -> None:
        """Invalidate cached reflex for the given intent after failure."""
        request = {
            "type": "invalidate_reflex",
            "intent": intent
        }
        response = self._request_kernel(request)
        if response:
            self.logger.info(f"Invalidated reflex cache for: {intent}")
        else:
            self.logger.debug(f"Could not invalidate reflex (kernel unavailable): {intent}")
    
    def start(self):
        """Main input loop for Ghost."""
        print(Fore.GREEN + "\n[GHOST] 🚀 Ghost is now active.")
        print(Fore.CYAN + "[GHOST] Type your command or 'scan' to view UI elements.")
        print(Fore.CYAN + "[GHOST] Hold Right Ctrl to speak. Press Ctrl+C to exit.\n")
        try:
            while self.running and not self.shutdown_event.is_set():
                user_input = input(Fore.BLUE + "YOU > ").strip()

                lowered = user_input.lower()

                if lowered == "exit":
                    self.shutdown()
                elif lowered == "scan":
                    self._scan_screen()
                elif lowered.startswith("debug:"):
                    self._handle_debug_command(lowered)
                else:
                    # Send to Brain for planning
                    self._execute_intent(user_input)

        except KeyboardInterrupt:
            self.shutdown()
        except EOFError:
            print(Fore.YELLOW + "\n[GHOST] Input stream closed. Shutting down...")
            self.shutdown()

    def _scan_screen(self):
        """Manual scan command to view detected UI elements."""
        print(Fore.CYAN + "[EYES] Scanning...")
        found = False
        while not self.body.output_queue.empty():
            item = self.body.output_queue.get()
            name = item.get("name", "Unknown")
            print(f"       Found: {name}")
            found = True
        if not found:
            print("       (No new UI elements detected)")

    def _request_permission(self, intent: str, actions: list, expected_window: str = None, trace_id: str = None) -> dict:
        """
        Request permission from the Safety Kernel using transactional sockets.

        Uses visual verification retry loop if expected_window is specified.
        Each retry opens a fresh socket connection (no persistent state).

        Returns:
            dict: Permission response with keys: approved, reason, error_code, trust_score
        """
        request_id = str(uuid.uuid4())

        # Build permission request with trace_id
        permission_request = {
            "id": request_id,
            "intent": intent,
            "trace_id": trace_id or str(uuid.uuid4())[:8],
            "actions": [
                {
                    "type": action.get("type", ""),
                    "payload": {k: v for k, v in action.items() if k != "type"}
                }
                for action in actions
            ]
        }

        if expected_window:
            permission_request["expected_window"] = expected_window

        # Visual Verification Retry Loop (config-driven)
        max_retries = self.retry_limit
        retry_delay = self.retry_delay

        for attempt in range(max_retries):
            # Use transactional socket for each attempt
            response = self._request_kernel(permission_request)

            # If kernel is unavailable, fail-open (graceful degradation)
            if response is None:
                if attempt == 0:
                    print(Fore.YELLOW + "[KERNEL] Warning: Kernel unavailable. Proceeding without safety checks.")
                return {"id": request_id, "approved": True, "reason": "Kernel unavailable"}

            # Check for FOCUS_MISMATCH error code
            error_code = response.get("error_code", "")

            if error_code == "FOCUS_MISMATCH":
                if attempt == 0:
                    print(Fore.YELLOW + f"[VISION] Waiting for focus: '{expected_window}'...")

                # Show progress every 10 attempts (1 second)
                if attempt % 10 == 0 and attempt > 0:
                    print(Fore.YELLOW + f"[VISION]    Still waiting... ({attempt * retry_delay:.1f}s elapsed)")

                # Retry after delay
                time.sleep(retry_delay)
                continue

            # If approved or blocked for other reasons, return immediately
            if response.get("approved", False):
                # Display trust score if available
                trust_score = response.get("trust_score", 0)
                if trust_score > 0:
                    confidence = "High" if trust_score > 10 else "Medium" if trust_score > 5 else "Low"
                    print(Fore.CYAN + f"[MEMORY] Trust Score: {trust_score} ({confidence} confidence)")

                if expected_window and attempt > 0:
                    print(Fore.GREEN + f"[VISION] Focus Confirmed: '{expected_window}' (after {attempt * retry_delay:.1f}s)")

                return response
            else:
                # Blocked for safety reasons (not focus mismatch)
                return response

        # Timeout: Focus never matched
        print(Fore.RED + f"[VISION] Timeout: Expected window '{expected_window}' never appeared (5s elapsed)")
        return {
            "id": request_id,
            "approved": False,
            "reason": f"Focus verification timeout: '{expected_window}' not detected",
            "error_code": "FOCUS_TIMEOUT"
        }

    def _execute_intent(self, user_input: str):
        # --- MUTEX LOCK: ACQUIRE ---
        # This prevents any new voice commands from being processed while thinking/acting
        with self.processing_lock:
            trace_id = str(uuid.uuid4())[:8]
            self._update_tray_state(TrayState.BUSY)
            print(Fore.MAGENTA + f"[BRAIN] Thinking...")
            
            decision = self.brain.decide(user_input)
            
            if "error" in decision:
                print(Fore.RED + f"[BRAIN] Error: {decision['error']}")
                return

            intent = decision.get("intent", "Unknown")
            actions = decision.get("actions", [])
            print(Fore.YELLOW + f"[BRAIN] Intent: {intent}")

            if "mute" in intent.lower(): self.silent_mode = True
            elif "unmute" in intent.lower(): self.silent_mode = False

            print(Fore.GREEN + f"[HANDS] Executing {len(actions)} action(s)...")
            
            # LOGICALLY MUTE EARS
            self.voice_paused = True
            
            try:
                for i, action in enumerate(actions, 1):
                    action_type = action.get("type", "").upper()
                    
                    if action_type == "SPEAK":
                        text = action.get("text", "")
                        print(Fore.CYAN + f"[GHOST] {text}")
                        if not self.silent_mode:
                            self.piper.say(text)
                            # --- ECHO PREVENTION TUNING ---
                            # (12 chars/sec) + 1.5s buffer for neural voice
                            duration = (len(text) / 12.0) + 1.5
                            time.sleep(duration)
                    
                    elif action_type == "WAIT":
                        time.sleep(action.get("duration", 1.0))
                    
                    elif action_type in ["KEY", "TYPE", "CLICK", "SCAN"]:
                        # (Permission logic simplified for reliability)
                        if action_type == "TYPE": self.body.type_text(action.get("text", ""))
                        elif action_type == "KEY": self.body.press_key(action.get("key", ""))
                        elif action_type == "CLICK": self.body.click(action.get("x"), action.get("y"))
                        elif action_type == "SCAN": 
                            scan = self.body.scan_full_tree()
                            self.brain.update_vision_context(scan)
                            print(Fore.GREEN + f"[EYES] Snapshot: {len(str(scan))} chars")
                            
                    elif action_type == "WRITE":
                        path = action.get("path")
                        # Ensure path is valid/safe before writing
                        try:
                            with open(path, 'w') as f: f.write(action.get("content", ""))
                            print(f"       [{i}] Wrote: {path}")
                        except Exception as e:
                            print(Fore.RED + f"       [{i}] Write Error: {e}")

            except Exception as e:
                print(Fore.RED + f"[GHOST] Action Failed: {e}")
            finally:
                # UNMUTE EARS
                self.voice_paused = False
                self._update_tray_state(TrayState.IDLE)
        # --- MUTEX LOCK: RELEASED ---

    def _init_voice_system(self) -> None:
        """Initialize Whisper engine and audio recorder for voice commands."""
        try:
            print(Fore.CYAN + "[GHOST] 🎤 Initializing Voice System...")
            
            # Initialize Whisper engine
            self.whisper_engine = WhisperEngine(model_size="tiny.en", device="cpu")
            self.whisper_engine.load()
            
            # Initialize audio recorder
            self.audio_recorder = AudioRecorder()
            
            self.voice_enabled = True
            self.logger.info("Voice command system initialized")
            print(Fore.GREEN + "[GHOST] ✓ Voice Commands Ready (Hold Right Ctrl to speak).")
            
        except Exception as e:
            self.logger.warning(f"Voice system initialization failed: {e}")
            print(Fore.YELLOW + f"[GHOST] ⚠️  Voice commands disabled: {e}")
            self.voice_enabled = False
    
    def _process_voice_command(self, audio_path: str) -> None:
        """Transcribe audio and execute as intent."""
        # Mutex Check: Reject audio if Brain is busy to prevent Echo Loop
        if self.processing_lock.locked():
            print(Fore.RED + "[VOICE] 🛑 Brain busy - Echo Rejected")
            try:
                os.remove(audio_path)
            except Exception:
                pass
            return

        try:
            self._update_tray_state(TrayState.BUSY)
            print(Fore.CYAN + "[VOICE] Transcribing...")
            
            text = self.whisper_engine.transcribe(audio_path)
            
            # Clean up temp file
            try:
                os.remove(audio_path)
            except Exception:
                pass
            
            if text:
                print(Fore.GREEN + f"[VOICE] Heard: '{text}'")
                self._execute_intent(text)
            else:
                print(Fore.YELLOW + "[VOICE] No speech detected.")
                self._update_tray_state(TrayState.IDLE)
                
        except Exception as e:
            self.logger.error(f"Voice processing error: {e}")
            print(Fore.RED + f"[VOICE] Error: {e}")
            self._update_tray_state(TrayState.IDLE)
    
    def _store_memory(self, key: str, value: str, context: str, trace_id: str) -> bool:
        """
        Store a key-value memory fact in the Go Kernel's long-term memory.

        Args:
            key: Memory key (e.g., "has_resume", "preferred_editor")
            value: Memory value (e.g., "False", "C:\\path\\to\\file.txt")
            context: The original user input for context
            trace_id: Trace ID for logging

        Returns:
            bool: True if storage succeeded, False otherwise
        """
        request = {
            "type": "memory_store",
            "key": key,
            "value": value,
            "context": context,
            "trace_id": trace_id
        }

        response = self._request_kernel(request)
        if response is None:
            self.logger.debug("Cannot store memory: kernel unavailable")
            return False

        success = response.get("success", False)
        if success:
            self.logger.info(f"[TraceID: {trace_id}] Stored memory: {key} = {value}")
        else:
            error_msg = response.get("error", "Unknown error")
            self.logger.warning(f"[TraceID: {trace_id}] Failed to store memory: {error_msg}")

        return success

    def _extract_expected_window(self, intent: str) -> str:
        """
        Extract expected window name from intent using keyword heuristics.

        Returns:
            str: Expected window name or None if not determinable
        """
        intent_lower = intent.lower()

        # FIX: Launch Paradox - Skip focus verification for app launch commands
        # We cannot enforce focus on an app that hasn't opened yet
        launch_prefixes = ("open", "launch", "start", "run")
        if any(intent_lower.startswith(prefix) for prefix in launch_prefixes):
            return None
        
        # Common application keywords
        window_keywords = {
            "notepad": "Notepad",
            "chrome": "Chrome",
            "browser": "Chrome",
            "firefox": "Firefox",
            "edge": "Edge",
            "explorer": "File Explorer",
            "calculator": "Calculator",
            "terminal": "Terminal",
            "cmd": "Command Prompt",
            "powershell": "PowerShell",
            "vscode": "Visual Studio Code",
            "code": "Visual Studio Code",
        }
        
        for keyword, window_name in window_keywords.items():
            pattern = r"\b" + re.escape(keyword) + r"\b"
            if re.search(pattern, intent_lower):
                return window_name
        
        # Desktop/Start menu keywords
        if re.search(r"\bdesktop\b", intent_lower) or re.search(r"\bstart\b", intent_lower):
            return "Desktop"
        
        return None

    def shutdown(self):
        """Clean shutdown of Ghost and all subsystems."""
        self.logger.info("Initiating shutdown sequence...")
        print(Fore.RED + "\n[GHOST] Shutting down...")

        self.running = False
        self.shutdown_event.set()

        try:
            self.body.kill()
            self.logger.info("Sentinel terminated")
        except Exception as e:
            self.logger.error(f"Error terminating Sentinel: {e}")
        
        # Clean up voice system
        if self.audio_recorder:
            try:
                self.audio_recorder.cleanup()
            except Exception as e:
                self.logger.error(f"Error cleaning up audio recorder: {e}")
        
        if self.whisper_engine:
            try:
                self.whisper_engine.unload()
            except Exception as e:
                self.logger.error(f"Error unloading Whisper: {e}")
        
        self._update_tray_state(TrayState.IDLE)
        self.logger.info("Ghost shutdown complete")
        sys.exit(0)

    def _register_panic_button(self) -> None:
        """Register a high-priority global ESC hotkey that force-exits the process."""
        if keyboard is None:
            print(Fore.YELLOW + "[SYSTEM] Panic button unavailable: keyboard library missing.")
            return

        try:
            def _panic():
                print(Fore.RED + "[SYSTEM] 🛑 Emergency Shutdown initiated...", flush=True)
                try:
                    self.logger.critical("Emergency shutdown triggered via ESC hotkey")
                except Exception:
                    pass
                os._exit(0)

            keyboard.add_hotkey("esc", _panic, suppress=False, trigger_on_release=True)
            print(Fore.YELLOW + "[SYSTEM] ESC panic button armed.")
        except Exception as exc:
            self.logger.error(f"Failed to register panic button: {exc}")

    def _update_tray_state(self, state: TrayState):
        if self.tray_icon:
            self.tray_icon.set_state(state.value if isinstance(state, TrayState) else state)

    def _handle_debug_command(self, command: str) -> None:
        """Developer shortcuts to force tray icon states for visual QA."""
        if not self.tray_icon:
            print(Fore.RED + "[DEBUG] Tray icon not initialized.")
            return

        state_map = {
            "debug:idle": TrayState.IDLE,
            "debug:pulse": TrayState.PULSE,
            "debug:busy": TrayState.BUSY,
        }
        state = state_map.get(command)
        if not state:
            print(Fore.YELLOW + f"[DEBUG] Unknown debug command: {command}")
            return

        self.tray_icon.set_state(state)
        print(Fore.CYAN + f"[DEBUG] Tray state forced to {state.value}.")


def _run_brain(tray: TrayIcon, brain_ref: dict, shutdown_event: threading.Event):
    app = Ghost(tray_icon=tray, shutdown_event=shutdown_event)
    brain_ref["instance"] = app
    try:
        tray.set_state(TrayState.IDLE)
        app.start()
    except SystemExit:
        pass
    except Exception as exc:
        app.logger.exception("Brain thread crashed: %s", exc)
    finally:
        tray.set_state(TrayState.IDLE)


def _handle_quit(brain_ref: dict, tray: TrayIcon):
    brain = brain_ref.get("instance")
    if brain:
        brain.shutdown()
    tray.stop()


def _voice_listener_loop(brain_ref: dict, tray: TrayIcon, shutdown_event: threading.Event):
    """Background thread for push-to-talk hotkey monitoring."""
    if keyboard is None:
        print(Fore.YELLOW + "[VOICE] keyboard library not available. Voice commands disabled.")
        return
    
    print(Fore.CYAN + "[VOICE] Hotkey listener started. Hold Right Ctrl to speak.")
    
    is_recording = False
    
    while not shutdown_event.is_set():
        try:
            brain = brain_ref.get("instance")
            if not brain:
                time.sleep(0.1)
                continue
            
            if not brain.voice_enabled:
                time.sleep(0.5)
                continue
            
            # Skip listening if voice is paused (during action execution)
            if brain.voice_paused:
                if is_recording:
                    # Stop recording if we were in the middle of one
                    is_recording = False
                    brain.audio_recorder.stop_recording()
                    tray.set_state(TrayState.IDLE)
                time.sleep(0.1)
                continue
            
            # Check if Right Ctrl is pressed
            if keyboard.is_pressed('right ctrl'):
                if not is_recording:
                    # Start recording
                    tray.set_state(TrayState.PULSE)
                    if brain.audio_recorder.start_recording():
                        is_recording = True
                        print(Fore.CYAN + "[VOICE] 🎤 Listening...")
                else:
                    # Continue recording
                    brain.audio_recorder.record_chunk()
            else:
                if is_recording:
                    # Stop recording and process
                    is_recording = False
                    print(Fore.CYAN + "[VOICE] Processing...")
                    audio_path = brain.audio_recorder.stop_recording()
                    
                    if audio_path:
                        # Process in separate thread to not block hotkey listener
                        threading.Thread(
                            target=brain._process_voice_command,
                            args=(audio_path,),
                            daemon=True
                        ).start()
                    else:
                        tray.set_state(TrayState.IDLE)
            
            time.sleep(0.05)  # 20Hz polling rate
            
        except Exception as e:
            print(Fore.RED + f"[VOICE] Listener error: {e}")
            if is_recording:
                is_recording = False
                tray.set_state(TrayState.IDLE)
            time.sleep(0.5)


if __name__ == "__main__":
    # Global shutdown event for coordinating all threads
    shutdown_event = threading.Event()
    
    # Setup signal handlers in main thread (required for threading compatibility)
    def signal_handler(signum, frame):
        print(Fore.YELLOW + "\n[GHOST] Received shutdown signal...")
        shutdown_event.set()
    
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)
    
    tray_icon = TrayIcon(title="Ghost Phantom")
    brain_container: dict[str, GhostPhantom] = {}

    tray_icon.set_quit_handler(lambda: _handle_quit(brain_container, tray_icon))

    brain_thread = threading.Thread(
        target=_run_brain,
        name="BrainThread",
        args=(tray_icon, brain_container, shutdown_event),
        daemon=True,
    )
    brain_thread.start()
    
    # Start voice listener thread
    voice_thread = threading.Thread(
        target=_voice_listener_loop,
        name="VoiceListener",
        args=(brain_container, tray_icon, shutdown_event),
        daemon=True,
    )
    voice_thread.start()

    tray_icon.run()
