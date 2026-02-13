# Ghost: The Hollow Prism

> "A sovereign, local-first AI system that lives in the machine."

![Ghost Terminal](https://via.placeholder.com/1200x600?text=Ghost+Dashboard+Preview)

## Vision

Ghost is a **voice-controlled, local-first AI architecture** designed to give users complete sovereignty over their data and digital interactions. Built for the **Mozilla Fellowship 2026** cohort, it demonstrates that advanced AI assistance does not require surrendering privacy to cloud providers.

### Core Constraints

- **Offline-Only:** No dependency on external APIs for core function.
- **Zero Telemetry:** No data leaves the device.
- **Consumer Hardware:** Runs on standard Laptops (MacBook Air M2 / Windows Surface).

## Features

- **Neural Terminal:** A premium, cinematic dashboard for interacting with the system.
- **Sovereign Brain:** Local LLM inference (Llama 3.1 8B / Mistral) via Ollama/Llama.cpp.
- **Sentinel Eye:** Privacy-focused screen context awareness (OCR/Vision).
- **Reflex Engine:** Low-latency voice command execution (Whisper/Silero).

## Tech Stack

- **Kernel:** Go (Golang) + gRPC
- **Dashboard:** React + Vite + Tailwind CSS + Framer Motion
- **Database:** SQLite (Embedded)
- **AI Runtime:** Ollama / Python Binding

## Installation

### Prerequisites

- Go 1.22+
- Node.js 20+
- GCC (for SQLite CGO)

### Quick Start

1. **Clone the repository**

   ```bash
   git clone https://github.com/enkae-dev/ghost.git
   cd ghost
   ```

2. **Build the landing page** (optional; for kernel to serve UI at `/`)

   ```powershell
   cd apps/landing && npm install && npm run build
   cd ../..
   .\scripts\build-static.ps1
   ```

   Or run the kernel API-only and deploy the landing separately (see `.context/FRONTEND_LAYOUT.md`).

3. **Start the Kernel**

   ```bash
   cd conscience_go
   go run main.go
   ```

4. **Access the System**
   Open your browser to `http://localhost:8080`.

## Architecture (High-Level)

```mermaid
graph TD
    User[User Voice/Input] --> Browser[Dashboard (React)]
    Browser -- HTTP/gRPC --> Gateway[Go Kernel Gateway]
    Gateway --> Service[Ghost Service]
    Service --> Brain[AI Model (Local)]
    Service --> Memory[SQLite DB]
    Service --> Sentinel[Screen/OS Context]
```

## License

MIT License. Sovereign forever.
