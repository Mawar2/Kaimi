package agent

import "context"

// Agent is the interface every agent in the system must implement.
// Callers dispatch work via Run and receive a uniform AgentResult.
type Agent interface {
	// Name returns the agent's identifier, matching AgentResult.AgentName.
	Name() string

	// Run executes the agent's task and returns a result.
	// The noticeID ties the result back to the triggering event (e.g. a GitHub issue number).
	Run(ctx context.Context, noticeID string) (*AgentResult, error)
}

// StubAgent is a no-op implementation of Agent used to validate the contract shape.
// It always returns AgentStatusSuccess and is safe to use in tests and wire-up checks.
type StubAgent struct {
	name string
}

// NewStubAgent creates a StubAgent with the given name.
func NewStubAgent(name string) *StubAgent {
	return &StubAgent{name: name}
}

// Name returns the stub's identifier.
func (s *StubAgent) Name() string { return s.name }

// Run returns a valid AgentResult with AgentStatusSuccess to prove the contract shape works.
func (s *StubAgent) Run(_ context.Context, noticeID string) (*AgentResult, error) {
	return &AgentResult{
		AgentName: s.name,
		Status:    AgentStatusSuccess,
		NoticeID:  noticeID,
		Summary:   "stub agent completed successfully",
		OutputRef: "",
		Flags:     []string{"stub"},
		Error:     "",
	}, nil
}
