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

---

## Frontend Layout (Source of Truth)

Keep the repo clean by separating **frontend source** from the kernel. The Go tree should not own React/UI source; it may serve **built** assets only.

| Purpose | Location | Serves |
|--------|----------|--------|
| **Landing (marketing)** | `apps/landing/` | Hero, protocol, features, install, docs links. Public site. |
| **Product UI (Neural Terminal)** | `apps/dashboard/` | Approvals, system state, modes, goals — the actual Ghost app that talks to `/v1/` or `/api/*`. |
| **Kernel** | `conscience_go/` | Go only. No frontend source. Optionally serves static files from a **build output** dir (e.g. `conscience_go/static/` populated at build time from `apps/landing` and/or `apps/dashboard`). |

**Current state:** Landing source is `apps/landing/`. Kernel serves from `conscience_go/static/` (build output). Run `./scripts/build-static.ps1` to build landing into static. Add `apps/dashboard/` when building the product UI; optionally remove legacy `conscience_go/dashboard/landing/` source once unused. See `.context/FRONTEND_LAYOUT.md` for full strategy and migration checklist.
