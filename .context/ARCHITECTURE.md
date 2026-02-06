# Ghost Tri-Brain Architecture

## Data Flow

The system follows a strict unidirectional control flow with bidirectional feedback.

### 1. Brain (Python)

- **Role:** Orchestrator & Planner.
- **Components:** Ollama (LLM), Whisper (STT), Piper (TTS).
- **Responsibility:** Interprets user intent, generates plans, sends commands to Body.

### 2. Conscience (Go)

- **Role:** Safety Kernel & Memory.
- **Components:** SQLite (Long-term Memory), Policy Engine.
- **Responsibility:** Validates Brain commands, enforces permissions, checks focus context.

### 3. Body (Rust)

- **Role:** Sentinel & Effector.
- **Components:** Windows Accessibility API (UI Automation), Input Simulation.
- **Responsibility:** Executes physical actions (type, click), captures UI state (sight).

## Interaction Loop

1. **Hear:** Body/Brain captures audio -> Whisper transcribes.
2. **Think:** Brain (LLM) plans actions based on intent + context.
3. **Check:** Brain sends planned actions to Conscience (Kernel).
4. **Approve:** Conscience validates against Policy + Focus State.
5. **Act:** Body executes approved physical actions.
