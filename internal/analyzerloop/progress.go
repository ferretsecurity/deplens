package analyzerloop

import "time"

// ProgressReporter presents a bounded human narrative. The journal and raw
// attempt output remain the durable diagnostic records.
type ProgressReporter interface {
	WorkItemStarted(WorkItem)
	AttemptStarted(Attempt)
	AgentStarted(Attempt, string, string)
	AgentMessage(Attempt, string)
	AgentCommand(Attempt, bool)
	AgentEdited(Attempt, []string)
	AgentHeartbeat(Attempt, time.Duration)
	AgentFinished(Attempt, time.Duration)
	ValidationStarted(Attempt)
	ValidationAccepted(Attempt, []string)
	AttemptFinished(Attempt, AttemptResult, error, time.Duration)
	WorkItemFinished(WorkItem, bool)
}

func report(progress ProgressReporter, action func(ProgressReporter)) {
	if progress != nil {
		action(progress)
	}
}
