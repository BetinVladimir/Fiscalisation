package domain

// OperationState is the lifecycle state of a fiscal operation.
type OperationState string

const (
	StateExecuting  OperationState = "EXECUTING"
	StateFiscalized OperationState = "FISCALIZED"
	StateUnknown    OperationState = "UNKNOWN"
	StateFailed     OperationState = "FAILED"
	StateRecovery   OperationState = "RECOVERY_REQUIRED"
)

// Terminal reports whether the state is a final, non-retryable state.
func (s OperationState) Terminal() bool {
	return s == StateFiscalized || s == StateUnknown || s == StateFailed
}
