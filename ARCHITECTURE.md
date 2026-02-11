# Ghost System Architecture

## Overview

Ghost uses a **Hexagonal Architecture** (Ports and Adapters) to ensure modularity and testability. The core business logic (The "Ghost") is isolated from external concerns like the database, UI, or specific AI model implementations.

## Directory Structure

```
ghost/
├── conscience_go/          # The Go Kernel (Central Nervous System)
│   ├── main.go             # Entry point & Wiring
│   ├── dashboard/          # React Frontend
│   ├── internal/
│   │   ├── core/           # Domain Entities & Interfaces
│   │   ├── service/        # Application Logic
│   │   ├── adapter/        # Implementations (SQL, Ollama, OS)
│   │   └── protocol/       # gRPC Protobuf Definitions
├── brain_python/           # Python AI Subsystem (Optional)
└── body_rust/              # Rust Native Hooks (Input/Output)
```

## Commmunication Protocols

### 1. The Nervous System (gRPC)

Internal communication between the Kernel and high-performance components (like the Screen Sentinel or Python AI Service) happens via **gRPC**.

- **Protocol:** `ghost.proto`
- **Transport:** HTTP/2 (gRPC)
- **Data:** Protocol Buffers (Binary)

### 2. The Dashboard Link (gRPC-Gateway)

The React Dashboard communicates with the Kernel via **HTTP/1.1 REST**, which is automatically translated to gRPC calls by the Kernel's embedded Gateway.

- **Endpoint:** `http://localhost:8080/v1/`
- **Format:** JSON

## Data Sovereignty (Storage)

All data is stored in a local **SQLite** database (`data/kernel.db`).

- **State:** Current system mode, active focus.
- **Memory:** Vector embeddings of user history (Future).
- **Intent Logs:** Audit trail of all AI actions for safety review.

## Safety Systems

### Kernel-Level Permission Gating

Every action requested by the AI (file access, shell command) must pass through the `RequestPermission` RPC. The Kernel checks this against a user-defined policy before execution.

### The Sentinel

A module that monitors screen content to provide context. It runs locally and filters PII (Personally Identifiable Information) before any text enters the context window.
