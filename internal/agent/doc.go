// Package agent provides the core interface contract for all Kaimi agents.
//
// The AgentResult type is the standardized return value for every agent in both
// Zone 1 (scheduled pipeline) and Zone 2 (per-proposal orchestration). Locking
// this contract early lets the Manager coordinate specialist agents without
// tight coupling and lets Timm's agent tickets (KAI-2+) start in parallel.
//
// Every agent — Hunter, Scorer, Outline, Writer, Final Review — must return an
// AgentResult. Use StubAgent as the starting template for any new agent.
package agent
