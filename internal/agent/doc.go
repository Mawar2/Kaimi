// Package agent defines the AgentResult contract — the single stable interface
// that every Kaimi agent returns.
//
// All agents in both Zone 1 (scheduled pipeline) and Zone 2 (per-proposal
// lifecycle) return an AgentResult. This contract is locked early so that
// downstream agents (Scorer, Manager, Outline, Writer, Final Review) can be
// built and tested independently without coordination.
//
// Key types:
//   - AgentResult — the return value of every agent's Run method
//   - Status — typed enum for agent completion states
//   - StubAgent — a minimal agent implementation used as a template
//
// Usage pattern for implementing a new agent:
//
//	type MyAgent struct{ name string }
//
//	func (a *MyAgent) Run(ctx context.Context, noticeID string) (agent.AgentResult, error) {
//	    // ... do work ...
//	    return agent.AgentResult{
//	        AgentName:   a.name,
//	        Status:      agent.StatusSuccess,
//	        NoticeID:    noticeID,
//	        Summary:     "...",
//	        CompletedAt: time.Now(),
//	    }, nil
//	}
package agent
