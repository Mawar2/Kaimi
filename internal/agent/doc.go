// Package agent defines the AgentResult contract shared by all Zone 2 agents
// (Manager, Outline, Writer, Final Review) in the Kaimi pipeline.
//
// Every agent in Zone 2 returns an AgentResult so coordinators can inspect
// outcomes, route decisions, and gate on human review without knowing the
// internals of each agent.
//
// Phase 0 provides the contract (types + a StubAgent) but does not build any
// live agents. Downstream tickets (KAI-2+) depend on this package.
package agent
