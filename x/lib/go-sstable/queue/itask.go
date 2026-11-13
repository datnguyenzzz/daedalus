package queue

type ITask interface {
	// Execute it's responsible action
	Execute() error
	// Release the allocated memory that the task holds
	Release()
	// OnHold until the task is ready to execute
	OnHold()
}
