// Package agent defines the AgentResult contract — the single stable interface
// that every Kaimi agent returns.
//
// All agents, regardless of zone (Zone 1 scheduled or Zone 2 orchestrated),
// return an AgentResult. This contract is locked early to unblock all downstream
// agent implementations (Scorer, Manager, Outline, Writer, Final Review).
//
// Usage: copy StubAgent as a starting template for new agents, replacing the
// Run method body with the agent's actual logic.
package agent
