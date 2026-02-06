# // Author: Enkae (enkae.dev@pm.me)
import requests
import time
import ollama
from sentence_transformers import SentenceTransformer
from typing import Optional, Dict, Any

API_URL = "http://localhost:3000/api/artifacts"
ENRICH_URL_TEMPLATE = "http://localhost:3000/api/artifacts/{}/enrich"
GOAL_URL = "http://localhost:3000/api/goal"
PROPOSE_URL = "http://localhost:3000/api/propose"
VECTOR_SEARCH_URL = "http://localhost:3000/api/search/vector"
ACTION_STATUS_URL_TEMPLATE = "http://localhost:3000/api/actions/{}"
STATE_URL = "http://localhost:3000/api/state"
SLEEP_INTERVAL = 5
PLANNER_INTERVAL = 2  # Poll for goals every 2 seconds
ACTION_POLL_INTERVAL = 0.5  # Poll action status every 500ms
STATE_POLL_INTERVAL = 1  # Poll state every 1 second

def load_model() -> SentenceTransformer:
    print("[PYTHON] Loading SentenceTransformer model...")
    model = SentenceTransformer('all-MiniLM-L6-v2')
    print("[PYTHON] Model loaded successfully.")
    return model

def fetch_artifacts() -> Optional[list[Dict[str, Any]]]:
    try:
        response = requests.get(API_URL, timeout=5)
        response.raise_for_status()
        return response.json()
    except requests.exceptions.RequestException as e:
        print(f"[PYTHON] Error fetching artifacts: {e}")
        return None

def _log_ollama_request(endpoint: str, payload: Dict[str, Any]) -> None:
    print(f"[OLLAMA][REQUEST] POST {endpoint}", flush=True)
    print(f"[OLLAMA][REQUEST] Payload: {payload}", flush=True)

def _log_ollama_response(response: Any) -> None:
    print(f"[OLLAMA][RESPONSE] Raw: {response}", flush=True)

def analyze_context(content: str) -> Dict[str, str]:
    try:
        payload = {
            'model': 'llama3',
            'messages': [
                {
                    'role': 'system',
                    'content': 'You are Engram, a sentient OS observer. Analyze the user\'s screen text. Return ONLY in this exact format:\nCLASSIFICATION: [WORK|PERSONAL|CODING|OTHER]\nSUMMARY: [one sentence]'
                },
                {
                    'role': 'user',
                    'content': content
                }
            ]
        }

        _log_ollama_request("http://localhost:11434/api/chat", payload)
        response = ollama.chat(**payload)
        _log_ollama_response(response)

        raw_response = response['message']['content']

        # Parse the response
        classification = "OTHER"
        summary = raw_response

        lines = raw_response.strip().split('\n')
        for line in lines:
            if line.startswith('CLASSIFICATION:'):
                classification = line.replace('CLASSIFICATION:', '').strip()
            elif line.startswith('SUMMARY:'):
                summary = line.replace('SUMMARY:', '').strip()

        return {
            'classification': classification,
            'summary': summary,
            'raw': raw_response
        }
    except Exception as e:
        print(f"[OLLAMA][ERROR] analyze_context failed: {e}", flush=True)
        return {
            'classification': 'ERROR',
            'summary': f'Ollama Offline: {e}',
            'raw': str(e)
        }

def enrich_artifact(artifact_id: str, classification: str, summary: str, embedding: list) -> bool:
    try:
        url = ENRICH_URL_TEMPLATE.format(artifact_id)
        payload = {
            'classification': classification,
            'summary': summary,
            'embedding': embedding
        }
        response = requests.post(url, json=payload, timeout=5)
        response.raise_for_status()
        return True
    except requests.exceptions.RequestException as e:
        print(f"[ERROR] Failed to enrich artifact {artifact_id}: {e}")
        return False

def vectorize_query(text: str, model) -> list:
    """Convert search text to vector embedding"""
    try:
        embedding = model.encode(text)
        return embedding.tolist()
    except Exception as e:
        print(f"[ERROR] Failed to vectorize query: {e}")
        return []

def fetch_state() -> str:
    """Fetch current application state from Go backend"""
    try:
        response = requests.get(STATE_URL, timeout=5)
        response.raise_for_status()
        data = response.json()
        return data.get('state', 'SHADOW')  # Default to SHADOW on error
    except Exception as e:
        print(f"[STATE] Error fetching state: {e}")
        return 'SHADOW'  # Default to safe mode on error

def dream_loop():
    model = load_model()
    print(f"[PYTHON] Starting dream loop (polling every {SLEEP_INTERVAL}s)...\n")

    while True:
        # Check state before processing
        state = fetch_state()
        if state == 'PAUSED':
            # In PAUSED mode, do nothing
            time.sleep(STATE_POLL_INTERVAL)
            continue

        # In ACTIVE or SHADOW mode, process artifacts
        artifacts = fetch_artifacts()

        if artifacts is None:
            print("[PYTHON] Failed to fetch artifacts. Retrying...")
            time.sleep(SLEEP_INTERVAL)
            continue

        if len(artifacts) == 0:
            print("[PYTHON] No artifacts found. Waiting...")
            time.sleep(SLEEP_INTERVAL)
            continue

        most_recent = artifacts[-1]
        content = most_recent.get("content", "")
        artifact_type = most_recent.get("type", "unknown")
        artifact_id = most_recent.get("id", "unknown")

        if not content:
            print("[PYTHON] Empty content. Skipping...")
            time.sleep(SLEEP_INTERVAL)
            continue

        embedding = model.encode(content)

        print(f"[PYTHON] Dreamt of: \"{content}\"")
        print(f"         Type: {artifact_type} | ID: {artifact_id[:8]}...")
        print(f"         Vector Size: {len(embedding)} dimensions")
        print(f"         Vector Sample: [{embedding[0]:.4f}, {embedding[1]:.4f}, {embedding[2]:.4f}, ...]")

        analysis = analyze_context(content)
        print(f"[LLM] Classification: {analysis['classification']}")
        print(f"[LLM] Summary: {analysis['summary']}")

        # Send enrichment back to Go
        if enrich_artifact(artifact_id, analysis['classification'], analysis['summary'], embedding.tolist()):
            print(f"[CORTEX] ✓ Synced thought to permanent memory (Hippocampus)\n")
        else:
            print(f"[CORTEX] ✗ Failed to sync thought to memory\n")

        time.sleep(SLEEP_INTERVAL)

# ===== AGENTIC PLANNER =====

def search_memory(goal_text: str, model: Optional[SentenceTransformer] = None) -> str:
    """
    Search long-term memory for relevant artifacts using vector similarity.
    Returns formatted string with relevant memories.
    """
    try:
        # Load model if not provided
        if model is None:
            model = load_model()

        # Vectorize the goal text
        embedding = model.encode(goal_text)
        vector = embedding.tolist()

        # POST to /api/search/vector
        payload = {
            "vector": vector,
            "limit": 5  # Top 5 most relevant memories
        }

        response = requests.post(VECTOR_SEARCH_URL, json=payload, timeout=5)
        response.raise_for_status()

        artifacts = response.json()

        if not artifacts or len(artifacts) == 0:
            return "No relevant memories found."

        # Format memories for LLM context
        memory_lines = []
        for idx, artifact in enumerate(artifacts):
            timestamp = artifact.get('timestamp', 'Unknown time')
            content = artifact.get('content', '')
            classification = artifact.get('classification', 'OTHER')
            summary = artifact.get('summary', '')

            memory_lines.append(f"Memory {idx + 1}: [{timestamp}] {classification}")
            if summary:
                memory_lines.append(f"  Summary: {summary}")
            memory_lines.append(f"  Content: {content}")

        result = "\n".join(memory_lines)
        print(f"[RAG] Found {len(artifacts)} relevant memories")
        return result

    except Exception as e:
        print(f"[RAG] Error searching memory: {e}")
        return "Memory search failed."

def fetch_active_goal() -> Optional[Dict[str, Any]]:
    """Poll for an active goal from the Go API"""
    try:
        response = requests.get(GOAL_URL, timeout=5)
        response.raise_for_status()
        goal_data = response.json()

        # If empty object, no goal is active
        if not goal_data or 'id' not in goal_data:
            return None

        return goal_data
    except requests.exceptions.RequestException as e:
        print(f"[PLANNER] Error fetching goal: {e}")
        return None

def generate_plan(goal_text: str, screen_context: str = "", memory_context: str = "") -> list[Dict[str, Any]]:
    """
    Generate a step-by-step plan to achieve the goal using LLM.
    Uses RAG to inject relevant memories into the planning process.
    Returns a list of action proposals.
    """
    try:
        # If no memory context provided, search for relevant memories
        if not memory_context:
            memory_context = search_memory(goal_text)

        system_prompt = f"""You are an Autonomous Agent Planner. Your goal is: '{goal_text}'.

RELEVANT MEMORIES:
{memory_context}

Use these memories to locate files, understand context, or recall previous interactions.

Available tools:
- TYPE_TEXT: Type text into the focused window. Payload: {{"text": "..."}}
- CLICK: Click at screen coordinates. Payload: {{"x": 100, "y": 200}}
- PRESS_KEY: Press a keyboard key. Payload: {{"key": "ENTER"}} (ENTER, ESC, SPACE, TAB, etc.)

Current screen context: {screen_context if screen_context else "No context available"}

Return ONLY a valid JSON array of steps. Each step must have:
{{"intent": "description", "action_type": "TYPE_TEXT|CLICK|PRESS_KEY", "payload": {{}}, "risk_score": 0-100}}

Example for "write hello in notepad":
[
  {{"intent": "Open Windows Start Menu", "action_type": "PRESS_KEY", "payload": {{"key": "LWIN"}}, "risk_score": 10}},
  {{"intent": "Type Notepad to search", "action_type": "TYPE_TEXT", "payload": {{"text": "notepad"}}, "risk_score": 5}},
  {{"intent": "Press Enter to launch", "action_type": "PRESS_KEY", "payload": {{"key": "ENTER"}}, "risk_score": 15}},
  {{"intent": "Type hello message", "action_type": "TYPE_TEXT", "payload": {{"text": "Hello"}}, "risk_score": 5}}
]

Now generate the plan for: '{goal_text}'"""

        payload = {
            'model': 'llama3',
            'messages': [
                {
                    'role': 'system',
                    'content': system_prompt
                },
                {
                    'role': 'user',
                    'content': f'Generate a plan for: {goal_text}'
                }
            ]
        }

        _log_ollama_request("http://localhost:11434/api/chat", payload)
        response = ollama.chat(**payload)
        _log_ollama_response(response)

        raw_response = response['message']['content']
        print(f"[PLANNER] Raw LLM response:\n{raw_response}\n")

        # Try to extract JSON from the response
        import json
        import re

        # Find JSON array in the response
        json_match = re.search(r'\[.*\]', raw_response, re.DOTALL)
        if json_match:
            plan_json = json_match.group(0)
            plan = json.loads(plan_json)

            print(f"[PLANNER] ✓ Generated {len(plan)} steps")
            return plan
        else:
            print("[PLANNER] ✗ Failed to extract JSON from LLM response")
            return []

    except Exception as e:
        print(f"[OLLAMA][ERROR] generate_plan failed: {e}", flush=True)
        print(f"[PLANNER] Error generating plan: {e}")
        return []

def propose_action(intent: str, payload: Dict[str, Any], risk_score: int, domain: str = "PLANNER") -> Optional[str]:
    """
    Send an action proposal to the Permission Kernel.
    Returns the action ID if successful, None otherwise.
    """
    try:
        action_payload = {
            "intent": intent,
            "risk_score": risk_score,
            "payload": payload,
            "domain": domain,
            "interaction_type": "PERMISSION"
        }

        response = requests.post(PROPOSE_URL, json=action_payload, timeout=5)
        response.raise_for_status()

        # Extract action ID from response
        action_data = response.json()
        action_id = action_data.get('id', None)

        print(f"[PLANNER] ✓ Proposed: {intent} (risk: {risk_score}) | ID: {action_id[:8] if action_id else 'unknown'}")
        return action_id
    except requests.exceptions.RequestException as e:
        print(f"[PLANNER] ✗ Failed to propose action: {e}")
        return None

def poll_action_status(action_id: str, timeout_seconds: int = 300) -> str:
    """
    Poll GET /api/actions/{id} until status is COMPLETED or FAILED.
    Waits indefinitely for WAITING_FOR_USER (user approval).
    Returns final status: COMPLETED, FAILED, or TIMEOUT.
    """
    url = ACTION_STATUS_URL_TEMPLATE.format(action_id)
    start_time = time.time()

    print(f"[SYNC] Waiting for action {action_id[:8]} to complete...")

    while True:
        try:
            response = requests.get(url, timeout=5)
            response.raise_for_status()

            action = response.json()
            status = action.get('status', 'UNKNOWN')

            if status == 'COMPLETED':
                print(f"[SYNC] ✓ Action {action_id[:8]} completed successfully")
                return 'COMPLETED'

            if status == 'FAILED':
                print(f"[SYNC] ✗ Action {action_id[:8]} failed")
                return 'FAILED'

            # If waiting for user, keep waiting (do not timeout)
            if status == 'WAITING_FOR_USER' or status == 'WAITING_FOR_CONTEXT':
                print(f"[SYNC] ⏸ Action {action_id[:8]} waiting for user...")
                time.sleep(ACTION_POLL_INTERVAL)
                continue

            # Check timeout only for non-user-waiting states
            elapsed = time.time() - start_time
            if elapsed > timeout_seconds:
                print(f"[SYNC] ⏱ Action {action_id[:8]} timed out after {timeout_seconds}s")
                return 'TIMEOUT'

            # Action is in progress (PENDING, APPROVED, EXECUTING)
            time.sleep(ACTION_POLL_INTERVAL)

        except requests.exceptions.RequestException as e:
            print(f"[SYNC] Error polling action status: {e}")
            time.sleep(ACTION_POLL_INTERVAL)
            continue

def update_goal_status(goal_id: str, status: str) -> bool:
    """Update goal status (mock implementation - would need new API endpoint)"""
    # For now, just log. In production, you'd call PUT /api/goal/{id}
    print(f"[PLANNER] Goal {goal_id[:8]} status: {status}")
    return True

def planner_loop():
    """
    Autonomous planner loop:
    1. Poll for active goals
    2. Generate plan using LLM
    3. Submit each step to Permission Kernel
    """
    print(f"[PLANNER] Starting Agentic Planner (polling every {PLANNER_INTERVAL}s)...\n")

    while True:
        # Check state before processing goals
        state = fetch_state()
        if state != 'ACTIVE':
            # Only process goals in ACTIVE mode
            if state == 'SHADOW':
                print("[PLANNER] 🟡 In SHADOW mode: Learning only, no action execution")
            elif state == 'PAUSED':
                print("[PLANNER] 🔴 In PAUSED mode: All processing disabled")
            time.sleep(STATE_POLL_INTERVAL)
            continue

        goal = fetch_active_goal()

        if not goal:
            # No active goal, wait and retry
            time.sleep(PLANNER_INTERVAL)
            continue

        goal_id = goal.get('id', 'unknown')
        goal_text = goal.get('goal', '')

        print(f"\n[PLANNER] 🎯 Active goal detected: {goal_text}")
        print(f"[PLANNER]    Goal ID: {goal_id[:8]}")

        # Update goal status to PLANNING
        update_goal_status(goal_id, 'PLANNING')

        # Generate plan
        plan = generate_plan(goal_text)

        if not plan:
            print("[PLANNER] ✗ Failed to generate plan. Marking goal as FAILED.")
            update_goal_status(goal_id, 'FAILED')
            time.sleep(PLANNER_INTERVAL)
            continue

        # Submit each step to Permission Kernel SEQUENTIALLY
        print(f"\n[PLANNER] Executing {len(plan)} steps SEQUENTIALLY...\n")

        for idx, step in enumerate(plan):
            intent = step.get('intent', 'Unknown action')
            payload = step.get('payload', {})
            risk_score = step.get('risk_score', 50)

            print(f"\n[PLANNER] Step {idx + 1}/{len(plan)}: {intent}")

            # Propose the action to the Permission Kernel
            action_id = propose_action(intent, payload, risk_score, domain="PLANNER")

            if not action_id:
                print(f"[PLANNER] ✗ Failed to propose step {idx + 1}, stopping plan execution")
                update_goal_status(goal_id, 'FAILED')
                break

            # CRITICAL: Wait for this action to complete before moving to next step
            final_status = poll_action_status(action_id, timeout_seconds=300)

            if final_status != 'COMPLETED':
                print(f"[PLANNER] ✗ Step {idx + 1} did not complete successfully (status: {final_status})")
                print(f"[PLANNER] Stopping plan execution")
                update_goal_status(goal_id, 'FAILED')
                break

            print(f"[PLANNER] ✓ Step {idx + 1}/{len(plan)} completed, proceeding to next step")

        else:
            # All steps completed successfully (for-else)
            print(f"\n[PLANNER] ✓ All {len(plan)} steps completed successfully")
            update_goal_status(goal_id, 'COMPLETED')

        # Wait before checking for next goal
        time.sleep(PLANNER_INTERVAL)

if __name__ == "__main__":
    import sys

    if len(sys.argv) > 1 and sys.argv[1] == "--planner":
        print("[CORTEX] Running in PLANNER mode")
        try:
            planner_loop()
        except KeyboardInterrupt:
            print("\n[PLANNER] Planner loop terminated by user.")
        except Exception as e:
            print(f"\n[PLANNER] Fatal error: {e}")
    else:
        print("[CORTEX] Running in DREAM mode")
        try:
            dream_loop()
        except KeyboardInterrupt:
            print("\n[PYTHON] Dream loop terminated by user.")
        except Exception as e:
            print(f"\n[PYTHON] Fatal error: {e}")
