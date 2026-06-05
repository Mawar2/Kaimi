# Kaimi Architecture

## Overview

Kaimi is a multi-agent orchestration system that processes GitHub Issues autonomously.
A Supervisor polls issues, classifies them by complexity, and routes them to worker pools.
Each worker invokes an LLM backend to implement the solution, then opens a pull request
for human review. No code merges automatically.

```
GitHub Issues
      │
      ▼
 Supervisor  ──── polls ───► Ticket Client (GitHub REST / MCP)
      │
      ▼
   Router  ──────────────── complexity classification (Simple / Medium / Complex)
      │
      ▼
 Task Queue  ─────────────── JSON-backed, atomic dequeue
      │
      ├──► GeminiFlash Workers  (simple tasks)
      ├──► GeminiPro Workers    (medium tasks)
      └──► Claude Workers       (complex tasks)
                │
                ▼
         LLM Backend  ──────── Execute(prompt) → response
                │
                ▼
          Parse response (branch name, PR number)
                │
                ▼
         Task → StatusReview
                │
                ▼
         Human Reviews PR
```

---

## Agent Interface Contract

Every agent in the system — real or stub — must implement the `Agent` interface defined
in `internal/agent` and return an `AgentResult`.  Locking this contract here unblocks all
downstream agent tickets; callers and the future Manager depend on this single shape.

### Package

```
internal/agent/
├── result.go   — AgentResult struct + AgentStatus type
└── stub.go     — Agent interface + StubAgent (proves the shape compiles and runs)
```

### AgentStatus values

| Value              | Meaning                                            |
|--------------------|----------------------------------------------------|
| `success`          | Agent completed work without errors                |
| `failed`           | Unrecoverable error; see `AgentResult.Error`       |
| `needs_human`      | Agent requires human intervention to proceed       |
| `ready_to_submit`  | Work is done and a PR is open for review           |

`AgentStatusFailed`, `AgentStatusSuccess`, and `AgentStatusReadyToSubmit` are terminal
(`.IsTerminal() == true`).  `AgentStatusNeedsHuman` is non-terminal — the Manager may
retry or escalate.

### AgentResult fields

```go
type AgentResult struct {
    AgentName string      `json:"agent_name"`   // which agent produced this
    Status    AgentStatus `json:"status"`        // outcome
    NoticeID  string      `json:"notice_id"`     // correlates back to triggering event
    Summary   string      `json:"summary"`       // human-readable one-liner
    OutputRef string      `json:"output_ref"`    // PR URL, branch name, or artifact path
    Flags     []string    `json:"flags"`         // qualifiers: "tdd_complete", etc.
    Error     string      `json:"error,omitempty"` // non-empty only on failure
}
```

### Agent interface

```go
type Agent interface {
    Name() string
    Run(ctx context.Context, noticeID string) (*AgentResult, error)
}
```

All real agents (code-writer, reviewer, etc.) implement this interface.
The `StubAgent` in `internal/agent/stub.go` always returns `AgentStatusSuccess`
and is used to validate the contract in tests and integration checks.

---

## Package Map

| Package                     | Responsibility                                    |
|-----------------------------|---------------------------------------------------|
| `internal/agent`            | AgentResult contract + Agent interface            |
| `internal/taskqueue`        | Task lifecycle; JSON-backed queue (Store pattern) |
| `internal/worker`           | ClaudeCodeWorker; wraps LLM backend               |
| `internal/llm`              | LLMBackend abstraction (swappable)                |
| `internal/orchestrator`     | Supervisor loop, Router, YAML config              |
| `internal/ticket`           | GitHub client (REST + MCP transports)             |
| `internal/conventions`      | CLAUDE.md / CONVENTIONS.md parser                 |
| `cmd/supervisor`            | Entry point; wires all components                 |

---

## Design Principles

- **Human-in-the-loop**: agents open PRs; humans merge. No auto-merge to main.
- **Convention-driven**: every agent reads `CLAUDE.md`/`CONVENTIONS.md` before acting.
- **Swappable backends**: `LLMBackend` interface lets Claude Code CLI be replaced by
  Antigravity/Vertex without changing worker logic.
- **Uniform result shape**: `AgentResult` is the single exit point for every agent,
  making the Manager's job straightforward regardless of which agent ran.
- **Cost-zero operation**: Phase 1 uses existing Claude Code and Antigravity subscriptions.
