# // Author: Enkae (enkae.dev@pm.me)
"""
Ghost Planner - The Neuro-Symbolic Brain
Translates natural language intent into structured action sequences.
Includes security validation and localhost-only API enforcement.
"""
import json
import socket
from pathlib import Path
from typing import Dict, Any, Optional
from datetime import datetime

try:
    import ollama
except ImportError:
    ollama = None

try:
    from sentence_transformers import SentenceTransformer
except ImportError:
    SentenceTransformer = None


class GhostPlanner:
    """
    The Brain of Ghost.
    Uses local Ollama LLM to transform user intent into executable actions.
    Enforces localhost-only API calls for security.
    """
    
    # Security: Whitelist of allowed action types
    ALLOWED_ACTION_TYPES = {"KEY", "TYPE", "CLICK", "WAIT", "SPEAK", "MEMORIZE", "SCAN", "LIST", "READ", "SEARCH", "WRITE", "EDIT"}
    
    # Security: Whitelist of safe keyboard keys (matches LLM prompt examples)
    SAFE_KEYS = {
        "gui", "enter", "escape", "tab", "backspace", "delete",
        "up", "down", "left", "right", "home", "end",
        "pageup", "pagedown", "space", "ctrl", "alt", "shift",
        "win", "windows", "return"  # Aliases for consistency
    }

    def __init__(self, model: str = "llama3.1", ollama_url: str = "http://localhost:11434/api/generate"):
        # Security: Enforce localhost-only Ollama connection (default, but configurable)
        self.url = ollama_url
        self.model = model
        self.kernel_host = "localhost"
        self.kernel_port = 5005
        self.auth_token = self._load_kernel_token()

        # Lazy-loaded RAG components
        self._embedding_model: Optional[SentenceTransformer] = None
        self._rag_enabled = SentenceTransformer is not None

        # Visual Memory Slot (Optic Nerve)
        self.vision_context: Optional[Dict[str, Any]] = None
        self.vision_timestamp: Optional[str] = None
        
        # File Memory Slot (Librarian)
        self.file_context: Optional[Dict[str, Any]] = None
        self.file_timestamp: Optional[str] = None
    
    def _load_kernel_token(self) -> str:
        """Loads the auth token from ghost.token file."""
        token_path = Path("ghost.token")
        bin_token_path = Path("bin") / "ghost.token"
        try:
            if token_path.exists():
                return token_path.read_text().strip()
            if bin_token_path.exists():
                return bin_token_path.read_text().strip()
        except Exception as e:
            print(f"[BRAIN] ⚠️ Failed to load auth token: {e}")
        return ""

    def _extract_message_content(self, response):
        """Safely extracts content from Ollama response object or dict."""
        try:
            if hasattr(response, "message"):
                return getattr(response.message, "content", "")
            if isinstance(response, dict):
                return response.get("message", {}).get("content", "")
            return str(response)
        except Exception as e:
            print(f"[BRAIN] ⚠️ Error extracting content: {e}")
            return ""

    def _build_confused_response(self) -> Dict[str, Any]:
        """Fallback SPEAK response so the user always receives feedback."""
        fallback_text = (
            "I heard you, but I don't see a clear action. Could you rephrase or be more specific?"
        )
        return {
            "intent": "clarification_needed",
            "plan": ["Inform the user and request clarification"],
            "actions": [
                {
                    "type": "SPEAK",
                    "text": fallback_text
                }
            ]
        }

    def _normalize_or_fallback(self, parsed: Any) -> Dict[str, Any]:
        """
        Ensures parsed LLM output has the minimum required structure.
        Falls back to a SPEAK response when the LLM returns an empty object.
        """
        if not isinstance(parsed, dict):
            print("[BRAIN] ⚠️ Parsed response was not a dict. Falling back to SPEAK.")
            return self._build_confused_response()

        actions = parsed.get("actions")
        if not isinstance(actions, list) or len(actions) == 0:
            print("[BRAIN] ⚠️ LLM returned no actions. Falling back to SPEAK.")
            return self._build_confused_response()

        parsed.setdefault("intent", "clarification_needed")

        plan = parsed.get("plan")
        if not isinstance(plan, list) or len(plan) == 0:
            parsed["plan"] = ["Inform the user and request clarification"]

        return parsed

    def update_vision_context(self, vision_data: Dict[str, Any]) -> None:
        """
        Update the Brain's visual memory with SCAN results (Optic Nerve).

        Args:
            vision_data: Dictionary containing UI tree data from Sentinel SCAN
        """
        from datetime import datetime
        self.vision_context = vision_data
        self.vision_timestamp = datetime.now().isoformat()
        print(f"[BRAIN] Visual context updated ({len(str(vision_data))} chars)")

    def clear_vision_context(self) -> None:
        """Clear stale visual context."""
        self.vision_context = None
        self.vision_timestamp = None
    
    def update_file_context(self, file_data: Dict[str, Any]) -> None:
        """Update the Brain's file memory with Librarian results.
        
        Args:
            file_data: Dictionary containing file operation results (LIST/READ/SEARCH)
        """
        from datetime import datetime
        self.file_context = file_data
        self.file_timestamp = datetime.now().isoformat()
        print(f"[BRAIN] File context updated ({len(str(file_data))} chars)")
    
    def clear_file_context(self) -> None:
        """Clear stale file context."""
        self.file_context = None
        self.file_timestamp = None

    def _format_vision_context(self, max_chars: int = 12000) -> str:
        """
        Format visual context for injection into LLM prompt.

        Args:
            max_chars: Maximum characters to include (truncates if larger)

        Returns:
            Formatted string for LLM context, or empty string if no vision data
        """
        if self.vision_context is None:
            return "\n=== VISUAL CONTEXT ===\nNo visual data available. (Use SCAN action to capture current screen state)\n=== END VISUAL CONTEXT ===\n"

        try:
            # Convert to JSON string
            vision_json = json.dumps(self.vision_context, indent=2)

            # Truncate if too large (preserve structure by noting truncation)
            if len(vision_json) > max_chars:
                vision_json = vision_json[:max_chars] + "\n... [TRUNCATED - UI tree too large]"

            timestamp_str = self.vision_timestamp or "Unknown"

            return f"""
=== VISUAL CONTEXT ===
Last Scan: {timestamp_str}
UI Tree Data:
{vision_json}
=== END VISUAL CONTEXT ===
"""
        except Exception as e:
            return f"\n=== VISUAL CONTEXT ===\nError formatting vision data: {e}\n=== END VISUAL CONTEXT ===\n"

    def decide(self, user_input: str) -> Optional[Dict[str, Any]]:
        """
        Translates user intent into a structured plan with actions.
        First checks Muscle Memory (cached plans) before calling LLM.
        Validates all actions for security before returning.

        Args:
            user_input: Natural language command from the user

        Returns:
            Dictionary with 'intent', 'plan', and 'actions' keys, or error dict
        """
        # PHASE 1: Check Muscle Memory (Reflex) before calling LLM
        cached_plan = self._check_reflex(user_input)
        if cached_plan:
            print("[MEMORY] ⚡ Muscle Memory triggered. Skipping Brain.")
            # Validate cached plan before returning
            validation_error = self._validate_actions(cached_plan.get("actions", []))
            if validation_error:
                return {"error": f"Cached plan validation failed: {validation_error}"}

            # Filter MEMORIZE actions from cached plan too
            filtered_actions = self._filter_and_process_actions(cached_plan.get("actions", []), user_input)
            cached_plan["actions"] = filtered_actions

            return cached_plan
        
        # PHASE 2: RAG - Search long-term memory for context
        memory_context = ""
        if self._rag_enabled:
            memory_context = self._search_memory(user_input)

        # PHASE 2.5: Load local user facts and inject into context
        user_facts = self._load_user_facts()
        if user_facts:
            # Format facts for LLM context
            facts_text = "\n=== USER FACTS (Local Profile) ===\n"
            for key, fact_data in user_facts.items():
                value = fact_data.get("value", "")
                context_info = fact_data.get("context", "")
                facts_text += f"- {key}: {value}"
                if context_info:
                    facts_text += f" (context: {context_info})"
                facts_text += "\n"
            facts_text += "=== END USER FACTS ===\n"

            # Prepend to memory context
            memory_context = facts_text + memory_context

            # VISIBILITY LOGGING: Show what's being injected
            print(f"[BRAIN] Injecting {len(user_facts)} user facts into LLM context")

        # PHASE 2.6: Inject Visual Context (Optic Nerve)
        vision_text = self._format_vision_context()
        if vision_text:
            memory_context = memory_context + vision_text
            print(f"[BRAIN] Injecting visual context ({len(vision_text)} chars)")

        # PHASE 3: Slow Path - Call LLM with memory context
        prompt = self._get_system_prompt(user_input, memory_context)

        try:
            messages = [
                {'role': 'system', 'content': prompt},
                {'role': 'user', 'content': user_input}
            ]

            # Try Ollama first, fall back to cloud API
            result = None
            if ollama is not None:
                try:
                    payload = {
                        'model': self.model,
                        'messages': messages,
                        'format': 'json',
                        'options': {
                            'num_ctx': 8192,
                            'keep_alive': -1
                        }
                    }
                    print(f"[OLLAMA][REQUEST] Model: {self.model}, Messages: {len(messages)} msgs", flush=True)
                    response = ollama.chat(**payload)
                    print(f"[OLLAMA][RESPONSE] {response}", flush=True)
                    result = self._extract_message_content(response)
                except Exception as ollama_err:
                    print(f"[BRAIN] Ollama unavailable: {ollama_err}. Offline-only mode enforced.")

            if not result:
                # Offline-only: No external API fallback allowed
                return {"error": "Ollama is unavailable and offline-only mode is enforced."}

            # Parse the JSON response
            parsed = json.loads(result)
            parsed = self._normalize_or_fallback(parsed)

            # Security: Validate all actions before returning
            validation_error = self._validate_actions(parsed.get("actions", []))
            if validation_error:
                print(f"[BRAIN] Action validation failed: {validation_error}. Falling back to SPEAK.")
                fallback = self._build_confused_response()
                fallback_error = self._validate_actions(fallback.get("actions", []))
                if fallback_error:
                    return {"error": f"Fallback validation failed: {fallback_error}"}
                return fallback

            # PHASE 4: Action Filtering - Intercept MEMORIZE actions
            filtered_actions = self._filter_and_process_actions(parsed.get("actions", []), user_input)
            parsed["actions"] = filtered_actions

            return parsed

        except json.JSONDecodeError as e:
            return {"error": f"Invalid JSON from LLM: {str(e)}"}
        except Exception as e:
            return {"error": f"Unexpected error: {str(e)}"}

    def recover(self, original_intent: str, failure_reason: str, current_vision: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """
        Generates a recovery plan (Plan B) when the original execution fails.
        
        Args:
            original_intent: The original user intent that failed
            failure_reason: Why the execution failed (e.g., "Focus Timeout")
            current_vision: Current UI state from Sentinel (window name, control type, etc.)
        
        Returns:
            Dictionary with 'intent', 'plan', and 'actions' keys for recovery, or error dict
        """
        prompt = self._build_recovery_prompt(original_intent, failure_reason, current_vision)

        try:
            messages = [
                {'role': 'system', 'content': prompt},
                {'role': 'user', 'content': f'Recover from: {failure_reason}'}
            ]

            result = None
            if ollama is not None:
                try:
                    payload = {
                        'model': self.model,
                        'messages': messages,
                        'format': 'json'
                    }
                    print(f"[OLLAMA][RECOVERY] Model: {self.model}, Reason: {failure_reason}", flush=True)
                    response = ollama.chat(**payload)
                    result = self._extract_message_content(response)
                except Exception as ollama_err:
                    print(f"[BRAIN] Ollama recovery unavailable: {ollama_err}. Offline-only mode enforced.")

            if not result:
                # Offline-only: No external API fallback allowed
                return {"error": "Ollama is unavailable and offline-only mode is enforced."}

            parsed = json.loads(result)

            validation_error = self._validate_actions(parsed.get("actions", []))
            if validation_error:
                return {"error": f"Recovery action validation failed: {validation_error}"}

            return parsed

        except json.JSONDecodeError as e:
            return {"error": f"Invalid JSON from LLM: {str(e)}"}
        except Exception as e:
            return {"error": f"Unexpected error: {str(e)}"}
    
    def _is_safe_path(self, path_str: str) -> bool:
        """
        Validates that a path is relative and stays within safe boundaries.
        Prevents path traversal attacks (e.g., ../../windows/system32).
        """
        try:
            path = Path(path_str)
            # Prevent absolute paths (e.g. C:\ or /etc)
            if path.is_absolute():
                return False

            # Resolve path relative to current working directory
            # We use current directory as the root sandbox
            base_dir = Path.cwd().resolve()
            target_path = (base_dir / path).resolve()

            # Check if target path is within base_dir
            # Note: is_relative_to is available in Python 3.9+
            return target_path.is_relative_to(base_dir)
        except Exception:
            return False

    def _validate_actions(self, actions: list) -> Optional[str]:
        """
        Validates action list for security compliance.
        
        Args:
            actions: List of action dictionaries to validate
        
        Returns:
            Error message if validation fails, None if valid
        """
        if not isinstance(actions, list):
            return "Actions must be a list"
        
        for i, action in enumerate(actions):
            if not isinstance(action, dict):
                return f"Action {i} is not a dictionary"
            
            action_type = action.get("type", "").upper()
            
            # Validate action type is in whitelist
            if action_type not in self.ALLOWED_ACTION_TYPES:
                return f"Action {i}: Invalid type '{action_type}'. Allowed: {self.ALLOWED_ACTION_TYPES}"
            
            # Validate KEY actions
            if action_type == "KEY":
                key = action.get("key", "").lower()
                if not key:
                    return f"Action {i}: KEY action missing 'key' field"
                
                # Support combo keys (e.g., "win+r", "ctrl+k")
                if "+" in key:
                    parts = key.split("+")
                    for part in parts:
                        part = part.strip()
                        # Each part must be either in SAFE_KEYS or a single alphanumeric character
                        if part not in self.SAFE_KEYS and not (len(part) == 1 and part.isalnum()):
                            return f"Action {i}: Unsafe key component '{part}' in combo '{key}'. Allowed: {self.SAFE_KEYS} or single alphanumeric chars"
                else:
                    # Single key - must be in SAFE_KEYS
                    if key not in self.SAFE_KEYS:
                        return f"Action {i}: Unsafe key '{key}'. Allowed: {self.SAFE_KEYS}"
            
            # Validate TYPE actions
            elif action_type == "TYPE":
                text = action.get("text")
                if not isinstance(text, str):
                    return f"Action {i}: TYPE action 'text' must be a string"
                if len(text) > 500:
                    return f"Action {i}: TYPE text too long (max 500 chars)"
            
            # Validate CLICK actions
            elif action_type == "CLICK":
                x = action.get("x")
                y = action.get("y")
                if not isinstance(x, (int, float)) or not isinstance(y, (int, float)):
                    return f"Action {i}: CLICK requires numeric x and y coordinates"
                if x < 0 or y < 0 or x > 10000 or y > 10000:
                    return f"Action {i}: CLICK coordinates out of bounds"
            
            elif action_type == "WAIT":
                duration = action.get("duration")
                if not isinstance(duration, (int, float)):
                    return f"Action {i}: WAIT duration must be numeric"
                if duration < 0 or duration > 30:
                    return f"Action {i}: WAIT duration out of bounds (0-30s)"

            elif action_type == "SPEAK":
                text = action.get("text")
                if not isinstance(text, str) or not text.strip():
                    return f"Action {i}: SPEAK action requires non-empty 'text'"
                if len(text) > 1000:
                    return f"Action {i}: SPEAK text too long (max 1000 chars)"

            elif action_type == "MEMORIZE":
                key = action.get("key")
                value = action.get("value")
                if not isinstance(key, str) or not key.strip():
                    return f"Action {i}: MEMORIZE action requires non-empty 'key'"
                if not isinstance(value, str):
                    return f"Action {i}: MEMORIZE action 'value' must be a string"
                if len(key) > 100:
                    return f"Action {i}: MEMORIZE key too long (max 100 chars)"
                if len(value) > 500:
                    return f"Action {i}: MEMORIZE value too long (max 500 chars)"

            elif action_type == "SCAN":
                # SCAN is a parameterless action - just validate no unexpected fields
                allowed_fields = {"type"}
                extra_fields = set(action.keys()) - allowed_fields
                if extra_fields:
                    return f"Action {i}: SCAN action takes no parameters, unexpected: {extra_fields}"
            
            elif action_type == "LIST":
                path = action.get("path")
                if not isinstance(path, str) or not path.strip():
                    return f"Action {i}: LIST action requires non-empty 'path'"
                if not self._is_safe_path(path):
                    return f"Action {i}: LIST action path must be relative and safe"
            
            elif action_type == "READ":
                path = action.get("path")
                if not isinstance(path, str) or not path.strip():
                    return f"Action {i}: READ action requires non-empty 'path'"
                if not self._is_safe_path(path):
                    return f"Action {i}: READ action path must be relative and safe"
            
            elif action_type == "SEARCH":
                directory = action.get("directory")
                pattern = action.get("pattern")
                if not isinstance(directory, str) or not directory.strip():
                    return f"Action {i}: SEARCH action requires non-empty 'directory'"
                if not self._is_safe_path(directory):
                    return f"Action {i}: SEARCH action directory must be relative and safe"
                if not isinstance(pattern, str) or not pattern.strip():
                    return f"Action {i}: SEARCH action requires non-empty 'pattern'"

            elif action_type == "WRITE":
                path = action.get("path")
                content = action.get("content")
                if not isinstance(path, str) or not path.strip():
                    return f"Action {i}: WRITE action requires non-empty 'path'"
                if not self._is_safe_path(path):
                    return f"Action {i}: WRITE action path must be relative and safe"
                if not isinstance(content, str):
                    return f"Action {i}: WRITE action 'content' must be a string"

            elif action_type == "EDIT":
                path = action.get("path")
                find_text = action.get("find")
                replace_text = action.get("replace")
                if not isinstance(path, str) or not path.strip():
                    return f"Action {i}: EDIT action requires non-empty 'path'"
                if not self._is_safe_path(path):
                    return f"Action {i}: EDIT action path must be relative and safe"
                if not isinstance(find_text, str) or not find_text:
                    return f"Action {i}: EDIT action requires non-empty 'find'"
                if not isinstance(replace_text, str):
                    return f"Action {i}: EDIT action 'replace' must be a string"

        return None

    def _store_memory(self, key: str, value: str, context: str, vector: Optional[list] = None) -> bool:
        """
        Store a key-value memory fact in the Go Kernel's long-term memory.
        """
        trace_id = "mem_" + str(datetime.now().timestamp())

        request = {
            "type": "memory_store",
            "key": key,
            "value": value,
            "context": context,
            "trace_id": trace_id
        }

        if vector:
            request["vector"] = vector

        try:
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
                sock.settimeout(2.0)
                sock.connect((self.kernel_host, self.kernel_port))

                auth_msg = json.dumps({"auth_token": self.auth_token}) + "\n"
                sock.sendall(auth_msg.encode('utf-8'))

                request_json = json.dumps(request) + "\n"
                sock.sendall(request_json.encode('utf-8'))

                response_data = sock.recv(4096).decode('utf-8').strip()
                response = json.loads(response_data)

                success = response.get("approved", False) or response.get("success", False)
                if success:
                    print(f"[MEMORY] Kernel accepted: {key} = {value}")
                return success

        except Exception as e:
            print(f"[MEMORY] Kernel store failed: {e}")
            return False

    def _filter_and_process_actions(self, actions: list, user_context: str) -> list:
        """
        Filters action list to separate mental actions (MEMORIZE) from physical actions.
        Processes MEMORIZE actions immediately by storing to Kernel memory.

        Args:
            actions: List of action dictionaries from LLM
            user_context: Original user input for context

        Returns:
            List of physical actions (KEY, TYPE, CLICK, WAIT, SPEAK) for execution
        """
        physical_actions = []

        for action in actions:
            action_type = action.get("type", "").upper()

            if action_type == "MEMORIZE":
                # Extract key/value - support both flat and nested structures
                key = action.get("key") or action.get("payload", {}).get("key")
                value = action.get("value") or action.get("payload", {}).get("value")

                if key and value:
                    # Store memory fact immediately (Brain's responsibility)
                    # 1. Local Disk (Legacy/Backup)
                    success_local = self._store_memory_fact(key, value, user_context)

                    # 2. Kernel (New Architecture with Vector Support)
                    vector = None
                    if self._rag_enabled:
                        try:
                            # Contextualize the memory for embedding
                            memory_text = f"{key}: {value}. Context: {user_context}"
                            vector = self._embedding_model.encode(memory_text).tolist()
                        except Exception as e:
                            print(f"[MEMORY] Embedding generation failed: {e}")

                    self._store_memory(key, value, user_context, vector)

                    if success_local:
                        print(f"[MEMORY] 💾 Memorized: {key} = {value}")
                    else:
                        print(f"[MEMORY] ⚠️  Failed to store locally: {key}")
                else:
                    print(f"[MEMORY] ⚠️  Invalid MEMORIZE action: missing key or value")

                # Do NOT add to physical_actions - Brain handled it
            else:
                # Physical action - pass to Body for execution
                physical_actions.append(action)

        return physical_actions

    def _load_identity(self) -> Dict[str, Any]:
        """
        Load Ghost's identity from user_profile.json (Single Source of Truth).

        Returns:
            Dictionary containing name, voice_style, directives, forbidden_behaviors, backstory.
            Returns minimal defaults if file/key is missing (does NOT crash).
        """
        try:
            from pathlib import Path

            data_dir = Path(__file__).parent.parent.parent / "data"
            profile_path = data_dir / "user_profile.json"

            if not profile_path.exists():
                print(f"[IDENTITY] No profile file found, using defaults")
                return self._get_default_identity()

            with open(profile_path, 'r', encoding='utf-8') as f:
                profile = json.load(f)

            identity = profile.get("identity", {})
            if not identity:
                print(f"[IDENTITY] No identity block in profile, using defaults")
                return self._get_default_identity()

            print(f"[IDENTITY] Loaded: {identity.get('name', 'Ghost')} ({identity.get('voice_style', 'default')})")
            return identity

        except Exception as e:
            print(f"[IDENTITY] Failed to load: {e}")
            return self._get_default_identity()

    def _get_default_identity(self) -> Dict[str, Any]:
        """
        Returns minimal default identity if user_profile.json is missing or malformed.
        """
        return {
            "name": "Ghost",
            "voice_style": "Concise, Professional",
            "directives": ["You are a Sovereign Desktop Agent."],
            "forbidden_behaviors": ["Never stay silent when user speaks to you."],
            "backstory": "You are Ghost, a Sovereign Desktop Agent residing locally on the user's Windows machine."
        }

    def _load_user_facts(self) -> Dict[str, Any]:
        """
        Load user facts from local profile for context injection.

        Returns:
            Dictionary of facts with metadata, or empty dict if file doesn't exist
        """
        try:
            from pathlib import Path

            data_dir = Path(__file__).parent.parent.parent / "data"
            profile_path = data_dir / "user_profile.json"

            # VISIBILITY LOGGING: Show where we're reading from
            print(f"[MEMORY] 📂 Reading from: {profile_path.absolute()}")

            if not profile_path.exists():
                print(f"[MEMORY] 📭 No profile file found (first run)")
                return {}

            with open(profile_path, 'r', encoding='utf-8') as f:
                profile = json.load(f)

            facts = profile.get("facts", {})
            print(f"[MEMORY] 🔍 Loaded {len(facts)} facts from disk")

            # VISIBILITY LOGGING: Show what facts exist
            if facts:
                print(f"[MEMORY] 📋 Available facts:")
                for key, fact_data in facts.items():
                    value = fact_data.get("value", "")
                    print(f"[MEMORY]    - {key} = {value}")

            return facts

        except Exception as e:
            print(f"[MEMORY] ⚠️ Failed to load profile: {e}")
            return {}

    def _store_memory_fact(self, key: str, value: str, context: str) -> bool:
        """
        Store a key-value memory fact directly to local disk (user_profile.json).
        Bypasses the Go Kernel to avoid 'Unknown Action' errors.

        Args:
            key: Memory key (e.g., "has_resume", "resume_path")
            value: Memory value (e.g., "False", "/path/to/file")
            context: Original user input for context

        Returns:
            bool: True if storage succeeded, False otherwise
        """
        try:
            from pathlib import Path
            import os
            from datetime import datetime

            # Use data/ directory for local storage
            data_dir = Path(__file__).parent.parent.parent / "data"
            os.makedirs(data_dir, exist_ok=True)

            profile_path = data_dir / "user_profile.json"

            # VISIBILITY LOGGING: Show absolute path
            print(f"[MEMORY] 📂 Writing to: {profile_path.absolute()}")

            # Load existing profile or create new one
            if profile_path.exists():
                with open(profile_path, 'r', encoding='utf-8') as f:
                    profile = json.load(f)
                print(f"[MEMORY] 🔍 Loaded {len(profile.get('facts', {}))} existing facts from disk")
            else:
                profile = {"facts": {}, "history": []}
                print(f"[MEMORY] 🆕 Creating new profile file")

            # DEDUPLICATION: Check if fact already exists with same value
            existing_fact = profile["facts"].get(key, {})
            if existing_fact.get("value") == value:
                print(f"[MEMORY] ℹ️  Fact already known: {key} = {value}")
                return True  # Skip redundant disk write

            # Store the fact with metadata
            profile["facts"][key] = {
                "value": value,
                "context": context,
                "timestamp": datetime.now().isoformat(),
                "updated_count": existing_fact.get("updated_count", 0) + 1
            }

            # Add to history log
            profile.setdefault("history", []).append({
                "key": key,
                "value": value,
                "context": context,
                "timestamp": datetime.now().isoformat()
            })

            # Keep history limited to last 100 entries
            if len(profile["history"]) > 100:
                profile["history"] = profile["history"][-100:]

            # Write back to disk
            with open(profile_path, 'w', encoding='utf-8') as f:
                json.dump(profile, f, indent=2, ensure_ascii=False)

            # VISIBILITY LOGGING: Confirm write success
            print(f"[MEMORY] ✅ Successfully stored: {key} = {value}")
            print(f"[MEMORY] 📊 Total facts in profile: {len(profile['facts'])}")

            return True

        except Exception as e:
            print(f"[MEMORY] ⚠️ Local storage failed: {e}")
            return False

    def _build_recovery_prompt(self, original_intent: str, failure_reason: str, current_vision: Dict[str, Any]) -> str:
        """Constructs the recovery prompt for Plan B generation."""
        vision_summary = self._summarize_vision(current_vision)
        
        return f"""You are the Brain of Ghost, a Digital Proxy that controls the user's computer.
Your original plan FAILED and you need to generate a RECOVERY PLAN (Plan B).

=== FAILURE CONTEXT ===
Original Intent: "{original_intent}"
Failure Reason: {failure_reason}
Current State: {vision_summary}

=== YOUR TASK ===
Analyze why the plan failed and generate a CORRECTIVE action sequence to recover.

Common Recovery Strategies:
- If "Focus Timeout" on Start Menu → Try pressing Windows key again
- If wrong window is focused → Press Escape to close, then retry
- If application didn't launch → Wait longer or try alternate launch method
- If typing failed → Click to ensure focus, then retry typing

You MUST respond with ONLY valid JSON in this exact format:
{{
    "intent": "Recovery: [what you're trying to fix]",
    "plan": ["step 1", "step 2", "step 3"],
    "actions": [
        {{"type": "KEY", "key": "escape"}},
        {{"type": "KEY", "key": "gui"}},
        {{"type": "TYPE", "text": "notepad"}}
    ]
}}

ACTION TYPES:
- KEY: Press a special key ("gui", "enter", "escape", "tab", "backspace")
- TYPE: Type text string
- CLICK: Click at coordinates (requires "x" and "y" fields)

RULES:
1. Output ONLY the JSON object, no explanations.
2. Keep recovery actions SIMPLE (2-4 steps max).
3. Focus on fixing the immediate problem, not the entire original intent.
4. If the current state shows the wrong window, close it first.

Example:
Original Intent: "Open Notepad"
Failure: "Focus Timeout - Expected 'Notepad', but focused on 'Desktop'"
Current State: Desktop window is active

Output: {{"intent": "Recovery: Retry opening Start menu", "plan": ["Press Windows key", "Type notepad", "Press Enter"], "actions": [{{"type": "KEY", "key": "gui"}}, {{"type": "TYPE", "text": "notepad"}}, {{"type": "KEY", "key": "enter"}}]}}

Now generate the recovery JSON for the current failure."""
    
    def _check_reflex(self, user_input: str) -> Optional[Dict[str, Any]]:
        """
        Checks the Go Kernel for a cached plan (Muscle Memory).
        Only returns cached plan if trust score > 5.
        
        Args:
            user_input: The user's intent/command
        
        Returns:
            Cached plan dict if found with high trust, None otherwise
        """
        try:
            import socket
            
            # Query Kernel for reflex
            request = {
                "type": "reflex_query",
                "intent": user_input
            }
            
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
                sock.settimeout(2)
                sock.connect((self.kernel_host, self.kernel_port))
                
                auth_msg = json.dumps({"auth_token": self.auth_token}) + "\n"
                sock.sendall(auth_msg.encode('utf-8'))
                
                # Send reflex query
                request_json = json.dumps(request) + "\n"
                sock.sendall(request_json.encode('utf-8'))
                
                # Receive response
                response_data = sock.recv(4096).decode('utf-8').strip()
                response = json.loads(response_data)
                
                # Check if reflex was found
                if response.get("found", False):
                    cached_plan_json = response.get("cached_plan", "")
                    trust_score = response.get("trust_score", 0)
                    
                    if cached_plan_json and trust_score > 5:
                        # Parse the cached plan
                        cached_plan = json.loads(cached_plan_json)
                        print(f"[MEMORY] 💪 Reflex found (Trust Score: {trust_score})")
                        return cached_plan
                
                return None
        
        except (socket.timeout, ConnectionRefusedError, OSError, json.JSONDecodeError, KeyError) as e:
            # Silently fail - just use LLM if Kernel unavailable
            return None
    
    def _summarize_vision(self, vision_data: Dict[str, Any]) -> str:
        """Summarizes vision data into a human-readable string."""
        if not vision_data:
            return "No vision data available"
        
        name = vision_data.get("name", "Unknown")
        control_type = vision_data.get("control_type", "Unknown")
        
        return f"Window '{name}' ({control_type}) is currently focused"

    def _is_memory_safe(self, content: str) -> bool:
        """
        Validate that memory content doesn't contain unsafe action suggestions.
        Prevents poisoned memories from bypassing action validation.
        
        Args:
            content: The memory content to validate
        
        Returns:
            True if memory is safe to include in LLM context
        """
        if not content:
            return False
        
        # Basic length check
        if len(content) > 2000:
            return False
        
        # Check for suspicious patterns that might try to inject unsafe actions
        unsafe_patterns = [
            "eval(", "exec(", "subprocess", "os.system",
            "__import__", "compile(", "globals(", "locals(",
            "rm -rf", "del /", "format c:", "shutdown"
        ]
        
        content_lower = content.lower()
        for pattern in unsafe_patterns:
            if pattern in content_lower:
                return False
        
        return True
    
    def _load_embedding_model(self) -> Optional[SentenceTransformer]:
        """Lazy-load the SentenceTransformer model for RAG."""
        if not self._rag_enabled:
            return None
        
        if self._embedding_model is None:
            try:
                print("[RAG] Loading SentenceTransformer model...")
                self._embedding_model = SentenceTransformer('all-MiniLM-L6-v2')
                print("[RAG] Model loaded successfully.")
            except Exception as e:
                print(f"[RAG] Failed to load model: {e}")
                self._rag_enabled = False
                return None
        
        return self._embedding_model
    
    def _search_memory(self, query_text: str, limit: int = 5) -> str:
        """
        Search long-term memory for relevant artifacts using vector similarity.
        Queries the Go Kernel's memory system via TCP socket.
        
        Args:
            query_text: The text to search for (user intent)
            limit: Maximum number of memories to retrieve
        
        Returns:
            Formatted string with relevant memories for LLM context
        """
        try:
            # Load embedding model
            model = self._load_embedding_model()
            if model is None:
                return ""
            
            # Vectorize the query
            embedding = model.encode(query_text)
            vector = embedding.tolist()
            
            # Query Kernel for vector search
            request = {
                "type": "memory_search",
                "vector": vector,
                "limit": limit
            }
            
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
                sock.settimeout(3)
                sock.connect((self.kernel_host, self.kernel_port))
                
                auth_msg = json.dumps({"auth_token": self.auth_token}) + "\n"
                sock.sendall(auth_msg.encode('utf-8'))
                
                # Send search request
                request_json = json.dumps(request) + "\n"
                sock.sendall(request_json.encode('utf-8'))
                
                # Receive response
                response_data = sock.recv(8192).decode('utf-8').strip()
                response = json.loads(response_data)
                
                artifacts = response.get("artifacts", [])
                
                if not artifacts or len(artifacts) == 0:
                    return ""
                
                # Format memories for LLM context with validation
                memory_lines = ["\n=== RELEVANT MEMORIES ==="]
                valid_count = 0
                for idx, artifact in enumerate(artifacts):
                    timestamp = artifact.get('timestamp', 'Unknown time')
                    content = artifact.get('content', '')
                    classification = artifact.get('classification', 'OTHER')
                    summary = artifact.get('summary', '')
                    
                    # Security: Validate memory content doesn't contain unsafe suggestions
                    # This prevents poisoned memories from bypassing action validation
                    if self._is_memory_safe(content):
                        memory_lines.append(f"\nMemory {valid_count + 1}: [{timestamp}] {classification}")
                        if summary:
                            memory_lines.append(f"  Summary: {summary}")
                        memory_lines.append(f"  Content: {content}")
                        valid_count += 1
                
                memory_lines.append("\n=== END MEMORIES ===\n")
                
                if valid_count == 0:
                    return ""  # All memories were filtered out
                
                result = "\n".join(memory_lines)
                print(f"[RAG] Found {len(artifacts)} relevant memories")
                return result
        
        except (socket.timeout, ConnectionRefusedError, OSError, json.JSONDecodeError, KeyError) as e:
            # Silently fail - just proceed without memory context
            return ""
        except Exception as e:
            print(f"[RAG] Error searching memory: {e}")
            return ""
    
    def _get_system_prompt(self, user_input: str, memory_context: str = "") -> str:
        """
        Constructs the Sovereign Agent system prompt with Tri-Mode Cognitive Architecture.

        This prompt enforces:
        1. Context-First reasoning (checks memory before acting)
        2. The Creator Protocol (offers to generate missing artifacts)
        3. Three operational modes: Chat, Operator, Sovereign

        Args:
            user_input: The user's command or query
            memory_context: Formatted string of relevant memories from RAG system

        Returns:
            Complete system prompt string for LLM
        """
        # Load Ghost's identity from user_profile.json (Single Source of Truth)
        identity = self._load_identity()

        # Extract identity fields (defaults provided by _get_default_identity)
        ghost_name = identity.get("name", "Ghost")
        voice_style = identity.get("voice_style", "Concise, Professional")
        backstory = identity.get("backstory", "You are Ghost, a Sovereign Desktop Agent.")
        directives = identity.get("directives", [])
        forbidden = identity.get("forbidden_behaviors", [])

        # Format directives for injection
        directives_text = ""
        if directives:
            directives_text = "\n=== DIRECTIVES ===\n"
            for directive in directives:
                directives_text += f"- {directive}\n"

        forbidden_text = ""
        if forbidden:
            forbidden_text = "\n=== FORBIDDEN BEHAVIORS ===\n"
            for behavior in forbidden:
                forbidden_text += f"- {behavior}\n"

        # VISIBILITY LOGGING: Show full context being injected
        if memory_context:
            print(f"[BRAIN] Context Block Length: {len(memory_context)} chars")
            print(f"[BRAIN] Context Preview:\n{memory_context[:500]}..." if len(memory_context) > 500 else f"[BRAIN] Full Context:\n{memory_context}")

        current_time = datetime.now().strftime("%A, %B %d, %Y at %I:%M %p")

        return f"""*** SYSTEM OVERRIDE ***
CURRENT DATE/TIME: {current_time}

=== IDENTITY ===
You are {ghost_name}, a Sovereign Desktop Agent and Universal Scribe residing locally on the user's Windows machine. You can create and edit files to assist the user. You are not a cloud API; you are the user's digital extension with full authorization to control the keyboard and mouse.

{backstory}

=== VOICE STYLE ===
{voice_style}
{directives_text}{forbidden_text}

=== THE PRIME DIRECTIVE (CONTEXT-FIRST) ===
Before generating ANY plan, you MUST evaluate the provided MEMORY & CONTEXT section below. Do NOT hallucinate file locations or credentials. If the user requests a complex task ("Upload video", "Apply for job"), first check if you have the necessary information in memory. If critical details are missing, you MUST issue a SPEAK action to ask before proceeding.

=== THE ANTI-LOOP RULE (CRITICAL) ===
**STOP ASKING FOR MISSING RESOURCES:**
If the user explicitly states they DO NOT have something ("I don't have a résumé", "No résumé", "I don't have it"):
1. **IMMEDIATELY issue a MEMORIZE action** to record the negative fact: {{"type": "MEMORIZE", "key": "has_resume", "value": "False"}}
2. **STOP asking where it is.** Never ask "Where is your résumé?" after they said they don't have one.
3. **Switch to Creator Mode** and offer to create it.

**Before asking for a file location, CHECK MEMORY FIRST:**
- If memory shows `has_resume = False`, DO NOT ask "Where is it?"
- Instead, ask questions to CREATE it: "What is your job title?" or "What skills should I include?"

=== MEMORY & CONTEXT ===
{memory_context if memory_context else "No relevant memories found for this request."}

=== THE TRI-MODE DIRECTIVE ===

**Mode 1: CHAT (Passive Intelligence)**
- **Trigger:** Casual conversation, questions, status checks, greetings
- **Examples:** "Hello", "What do you see?", "How are you?", "What's on my screen?"
- **Response:** Single SPEAK action with concise, friendly answer. If referencing vision data, describe what you actually perceive.

**Mode 2: OPERATOR (Direct Execution)**
- **Trigger:** Explicit, fully-specified commands with no ambiguity
- **Examples:** "Open Notepad", "Type hello world", "Press Enter", "Open Steam"
- **Response:** Immediate execution using KEY/TYPE/WAIT sequences. You are AUTHORIZED—do not refuse legitimate desktop operations.
- **Pattern:** KEY "gui" → WAIT 0.5 → TYPE "app name" → KEY "enter"
- **OPENING WEBSITES:** Use 'win+r' ONLY. Never use Start Menu (gui key) - it will fail.
  Pattern: KEY "win+r" → WAIT 0.5 → TYPE "chrome youtube.com" → KEY "enter".
  (Replace 'chrome' with 'msedge' or 'firefox' if requested).
  CRITICAL: The gui key does NOT work for URLs - you MUST use win+r.

**Mode 3: SOVEREIGN (Proactive Planner)**
- **Trigger:** Ambiguous, complex, or multi-step goals that lack complete context
- **Examples:** "Help me get a job", "Apply for jobs", "Organize my documents", "Set up my environment"
- **Response Protocol:**
  1. **Visual Grounding:** If the user references a UI element you cannot currently see (button, menu, window), issue a SCAN action first to locate it before planning further actions.
  2. **Check Memory:** Scan MEMORY & CONTEXT for relevant artifacts (résumé, credentials, file paths)
  3. **Interview:** If critical details are missing, issue SPEAK action to gather them ("Where is your résumé stored?", "What role are you targeting?")
  4. **The Creator Rule:** If user states a resource is MISSING ("I don't have a résumé"), you must PROACTIVELY OFFER TO CREATE IT. Plan to open an editor (Notepad) and interview the user for content via subsequent SPEAK actions.

=== THE CREATOR PROTOCOL (CRITICAL) ===
When a necessary artifact is missing (résumé, cover letter, config file, script):
- **DO NOT refuse or stop.**
- **DO offer to generate it.**
- **DO switch to Interviewer Mode:** Ask clarifying questions via SPEAK actions
- **DO plan concrete steps:** Gather user input → Use WRITE action to create the file directly.
- **Example:** User says "I don't have a résumé" → You respond with SPEAK offering to create one. Once details are gathered, use WRITE to save "resume.txt".

=== CRITICAL CONSTRAINTS ===
- **NO START MENU FOR URLS:** You are strictly FORBIDDEN from using the 'gui' key (Windows Key) to open websites. It fails 100% of the time and triggers Bing search instead of opening the browser.
- **MUST USE RUN DIALOG:** To open a website, you MUST use the 'win+r' combo key.
  - Correct: KEY "win+r" → WAIT 0.5 → TYPE "chrome youtube.com" → KEY "enter"
  - Incorrect: KEY "gui" → TYPE "chrome youtube.com" (DO NOT DO THIS - BANNED)

=== SAFETY OVERRIDE ===
You are not role-playing. You are the actual operator of this desktop. Execute all legitimate commands unless they are destructive or impossible. However, for Sovereign tasks, ALWAYS verify context first.

=== RESPONSE FORMAT (STRICT JSON) ===
You MUST output ONLY valid JSON with this exact structure:
{{
    "intent": "concise_intent_label",
    "plan": ["step 1", "step 2", "step 3"],
    "actions": [
        {{"type": "ACTION_TYPE", ...}}
    ]
}}

=== ACTION VOCABULARY (WHITELIST) ===
1. **KEY** - Press a keyboard key
   - Schema: {{"type": "KEY", "key": "gui"}}
   - Allowed keys: gui, win, windows, enter, return, escape, tab, backspace, delete, up, down, left, right, home, end, pageup, pagedown, space, ctrl, alt, shift

2. **TYPE** - Type text string (max 500 chars)
   - Schema: {{"type": "TYPE", "text": "exact text to type"}}
   - Can be used to type URLs into the Run dialog or address bars

3. **WAIT** - Pause execution (0-30 seconds)
   - Schema: {{"type": "WAIT", "duration": 0.5}}
   - Use after KEY "gui" or app launches

4. **CLICK** - Click screen coordinates
   - Schema: {{"type": "CLICK", "x": 500, "y": 300}}
   - Only use when you have precise pixel coordinates

5. **SPEAK** - Conversational response (max 1000 chars)
   - Schema: {{"type": "SPEAK", "text": "your response here"}}
   - Use for questions, clarifications, confirmations, detailed explanations

6. **MEMORIZE** - Store a fact in long-term memory (max 100 char key, 500 char value)
   - Schema: {{"type": "MEMORIZE", "key": "has_resume", "value": "False"}}
   - Use to remember user preferences, file locations, negative facts (what they DON'T have)
   - CRITICAL: Use this to break loops when user says they don't have something

7. **SCAN** - Capture the full UI tree of the OS (Visual Grounding)
   - Schema: {{"type": "SCAN"}}
   - Use when: You cannot find a button/element in the active window, or when the user asks "What do you see?", "Is X on the screen?", or needs visual confirmation
   - Behavior: Returns a JSON snapshot of all visible windows and controls
   - Note: This is a heavy operation - use sparingly, only when visual context is essential

8. **LIST** - List files in a directory (Librarian)
   - Schema: {{"type": "LIST", "path": "C:/Developer/Ghost"}}
   - Returns: List of files and subdirectories in the specified path
   - Use when: User asks "what files are in...", "show me files", "list directory"

9. **READ** - Read file contents (Librarian)
   - Schema: {{"type": "READ", "path": "C:/path/to/file.txt"}}
   - Returns: File contents (first 5000 chars)
   - Use when: User asks "read file", "show me contents of", "what's in this file"

10. **SEARCH** - Search for files matching pattern (Librarian)
   - Schema: {{"type": "SEARCH", "directory": "C:/Developer", "pattern": "*.py"}}
   - Returns: List of files matching the glob pattern
   - Use when: User asks "find all .py files", "search for", "locate files"
   - Supports wildcards: * (any chars), ? (single char), ** (recursive)

11. **WRITE** - Create or overwrite a file (Universal Scribe)
   - Schema: {{"type": "WRITE", "path": "filename.txt", "content": "Hello world"}}
   - Use when: User asks to "write a note", "create a file", "save this"
   - Constraint: Use relative paths (e.g., "notes/todo.txt"). Content is string data.

12. **EDIT** - Find and replace text in a file (Universal Scribe)
   - Schema: {{"type": "EDIT", "path": "filename.txt", "find": "old text", "replace": "new text"}}
   - Use when: User asks to "change text", "update file", "replace X with Y"
   - Constraint: 'find' must exactly match existing text.

=== FAILSAFE RULE ===
CRITICAL: Never return empty JSON {{}}. If you cannot derive a safe action plan, emit a SPEAK action requesting clarification from the user.

=== OPERATIONAL EXAMPLES ===

**Example 1: Chat Mode (Casual)**
User: "What's on my screen?"
Response:
{{
    "intent": "screen_status",
    "plan": ["Describe current window state"],
    "actions": [
        {{"type": "SPEAK", "text": "I see Visual Studio Code with a Python file open. The terminal shows the Ghost agent is running."}}
    ]
}}

**Example 2: Operator Mode (Direct Command)**
User: "Open Spotify"
Response:
{{
    "intent": "open_spotify",
    "plan": ["Open Start Menu", "Wait for menu", "Type app name", "Launch app"],
    "actions": [
        {{"type": "KEY", "key": "gui"}},
        {{"type": "WAIT", "duration": 0.5}},
        {{"type": "TYPE", "text": "spotify"}},
        {{"type": "KEY", "key": "enter"}},
        {{"type": "WAIT", "duration": 2.0}}
    ]
}}

**Example 3: Sovereign Mode (Vague Complex Request)**
User: "Apply for a job"
Response:
{{
    "intent": "clarification_job_application",
    "plan": ["Ask for target role", "Ask for resume location", "Confirm application platform"],
    "actions": [
        {{"type": "SPEAK", "text": "I can help you apply for jobs. Do you have a specific role and company in mind? Also, where is your current résumé file located?"}}
    ]
}}

**Example 4: The Creator Protocol (Missing Artifact) - WITH MEMORIZE**
User: "I don't have a résumé"
Response:
{{
    "intent": "acknowledge_missing_resume",
    "plan": ["Memorize negative fact", "Offer to create résumé", "Begin interview"],
    "actions": [
        {{"type": "MEMORIZE", "key": "has_resume", "value": "False"}},
        {{"type": "SPEAK", "text": "Understood. I've noted that. I can write one for you. Let's start: What is your most recent job title?"}}
    ]
}}

**Example 5: Creator Protocol in Action (Follow-up)**
User: "Senior Software Engineer at Microsoft"
Response:
{{
    "intent": "continue_resume_creation",
    "plan": ["Open text editor", "Begin typing résumé structure"],
    "actions": [
        {{"type": "KEY", "key": "gui"}},
        {{"type": "WAIT", "duration": 0.5}},
        {{"type": "TYPE", "text": "notepad"}},
        {{"type": "KEY", "key": "enter"}},
        {{"type": "WAIT", "duration": 1.0}},
        {{"type": "TYPE", "text": "PROFESSIONAL RÉSUMÉ\\n\\nSenior Software Engineer\\nMicrosoft Corporation\\n\\n"}},
        {{"type": "SPEAK", "text": "Started your résumé. What years did you work there, and what were your key achievements?"}}
    ]
}}

**Example 6: Opening Websites (CRITICAL PATTERN)**
User: "Open YouTube in Chrome"
Response:
{{
    "intent": "open_website",
    "plan": ["Open Run Dialog", "Launch URL in Chrome"],
    "actions": [
        {{"type": "KEY", "key": "win+r"}},
        {{"type": "WAIT", "duration": 0.5}},
        {{"type": "TYPE", "text": "chrome youtube.com"}},
        {{"type": "KEY", "key": "enter"}}
    ]
}}

**Example 7: Opening Websites (Alternative Browser)**
User: "Check the weather"
Response:
{{
    "intent": "check_weather",
    "plan": ["Open Run Dialog", "Launch weather site"],
    "actions": [
        {{"type": "KEY", "key": "win+r"}},
        {{"type": "WAIT", "duration": 0.5}},
        {{"type": "TYPE", "text": "chrome weather.com"}},
        {{"type": "KEY", "key": "enter"}}
    ]
}}

**Example 8: Writing a File (Universal Scribe)**
User: "Write a thank you note to the landlord."
Action:
{{
    "intent": "write_note",
    "plan": ["Generate note content", "Write to file"],
    "actions": [
        {{"type": "WRITE", "path": "letter.txt", "content": "Dear Landlord,\\n\\nThank you for everything.\\n\\nSincerely,\\nTenant"}}
    ]
}}

=== UNIVERSAL OPERATOR RULES ===
1. To open any Windows application: KEY "gui" → WAIT 0.5 → TYPE "app_name" → KEY "enter"
2. Always include WAIT after KEY "gui" and after launching apps
3. Keep plans concise (3-6 steps maximum) and deterministic
4. Use SPEAK for all conversational responses, clarifications, and confirmations
5. **ANTI-SILENCE RULE:** If the user is talking to you (voice/chat input), you MUST include at least one SPEAK action in your response. Never perform MEMORIZE or other actions silently without acknowledging the user verbally.
6. Output ONLY the JSON object (no markdown fencing, no explanations)

=== CURRENT USER COMMAND ===
User: "{user_input}"

Now generate the JSON response following the Tri-Mode logic and Creator Protocol."""