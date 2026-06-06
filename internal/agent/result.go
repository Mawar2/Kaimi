// Package agent defines shared types for Kaimi's agent pipeline.
//
// Every agent in the Kaimi system follows the same contract: it accepts typed
// input, produces typed output, and returns a Result that describes the
// outcome. The Coordinator (Manager in Zone 2) orchestrates agents by reading
// their outputs and feeding inputs. Agents never call each other directly.
package agent

import "time"

// Result describes the outcome of a single agent invocation.
//
// Every agent Run() call returns a *Result alongside its typed output.
// Callers should check Status before using the output value. A nil
// opportunity or other expected failure is represented as Status "failed"
// rather than a non-nil error; errors are reserved for unexpected system
// failures (I/O, network, etc.).
type Result struct {
	AgentName string        // Name of the agent that produced this result
	Status    string        // "success" or "failed"
	Summary   string        // Human-readable description of what happened
	Duration  time.Duration // How long the agent ran
}
