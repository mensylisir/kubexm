package task

// Task defines the methods that all concrete task types must implement.
type Task interface {
	// Name returns the designated name of the task.
	Name() string

	// Description provides a brief summary of what the task does.
	Description() string

	// IsRequired determines if the task needs to generate a plan.
	IsRequired(ctx TaskContext) (bool, error)

	// Plan generates an ExecutionFragment for this task.
	Plan(ctx TaskContext) (*ExecutionFragment, error)
}
