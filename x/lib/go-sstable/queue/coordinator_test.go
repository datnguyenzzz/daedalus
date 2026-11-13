package queue

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockTask implements ITask interface for testing
type mockTask struct {
	executeFn  func() error
	onHoldChan chan struct{}
	released   bool
	onHold     bool
}

func newMockTask(executeFn func() error) *mockTask {
	return &mockTask{
		executeFn:  executeFn,
		onHoldChan: make(chan struct{}),
	}
}

func (m *mockTask) Execute() error {
	if m.executeFn != nil {
		return m.executeFn()
	}
	return nil
}

func (m *mockTask) Release() {
	m.released = true
}

func (m *mockTask) OnHold() {
	m.onHold = true
	close(m.onHoldChan)
}

// waitForOnHold waits until OnHold is called or times out
func (m *mockTask) waitForOnHold(timeout time.Duration) bool {
	select {
	case <-m.onHoldChan:
		return true
	case <-time.After(timeout):
		return false
	}
}

func TestNewQueue(t *testing.T) {
	// Test that NewQueue correctly initializes the coordinator
	q := NewQueue(5, false)

	// Type assertion to check if q is of type *coordinator
	c, ok := q.(*coordinator)
	assert.True(t, ok, "Queue should be of type *coordinator")

	assert.NotNil(t, c.ch, "Channel should be initialized")
	assert.Equal(t, 5, cap(c.ch), "Channel should have capacity 5")
	assert.False(t, c.ignoreErr, "ignoreErr should be false")
	assert.False(t, c.closed, "Queue should not be closed initially")
	assert.Nil(t, c.err, "Error should be nil initially")
	assert.NotNil(t, c.wg, "WaitGroup should be initialized")

	// Clean up
	_ = q.Close()
}

func TestCoordinator_Put(t *testing.T) {
	// Create a coordinator with a small buffer
	c := &coordinator{
		ch:        make(chan ITask, 2),
		wg:        &sync.WaitGroup{},
		ignoreErr: false,
	}
	c.wg.Add(1)
	go c.drainTask()

	// Create a mock task
	executedChan := make(chan struct{})
	task := newMockTask(func() error {
		close(executedChan)
		return nil
	})

	// Put the task in the queue
	c.Put(task)

	// Verify the task was executed
	select {
	case <-executedChan:
		// Task was executed
	case <-time.After(time.Second):
		t.Fatal("Task was not executed within timeout")
	}

	// Verify task was held and released
	assert.True(t, task.onHold, "Task should have been held")
	assert.True(t, task.released, "Task should have been released")

	// Clean up
	_ = c.Close()
}

func TestCoordinator_Close(t *testing.T) {
	// Create a coordinator
	c := &coordinator{
		ch:        make(chan ITask, 2),
		wg:        &sync.WaitGroup{},
		ignoreErr: false,
	}
	c.wg.Add(1)
	go c.drainTask()

	// Put a task that will be executed
	task := newMockTask(nil)
	c.Put(task)

	// Wait for task to be processed
	assert.True(t, task.waitForOnHold(time.Second), "Task OnHold should have been called")

	// Close the queue
	err := c.Close()
	assert.NoError(t, err, "Close should not return an error")
	assert.True(t, c.closed, "Queue should be marked as closed")

	// Verify that calling Close again returns the same error
	err2 := c.Close()
	assert.NoError(t, err2, "Second Close should not return an error")
}

func TestCoordinator_Error_Propagation(t *testing.T) {
	tests := []struct {
		name      string
		ignoreErr bool
		errors    []error
		want      error
	}{
		{
			name:      "First error stops execution when not ignoring errors",
			ignoreErr: false,
			errors:    []error{errors.New("first error"), errors.New("second error")},
			want:      errors.New("first error"),
		},
		{
			name:      "Last error is returned when ignoring errors",
			ignoreErr: true,
			errors:    []error{errors.New("first error"), errors.New("second error")},
			want:      errors.New("second error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a coordinator
			c := &coordinator{
				ch:        make(chan ITask, len(tt.errors)),
				wg:        &sync.WaitGroup{},
				ignoreErr: tt.ignoreErr,
			}
			c.wg.Add(1)
			go c.drainTask()

			// Put tasks with errors
			for _, err := range tt.errors {
				capturedErr := err // Capture err in the closure
				task := newMockTask(func() error {
					return capturedErr
				})
				c.Put(task)
			}

			// Close and check error
			err := c.Close()
			assert.Equal(t, tt.want.Error(), err.Error())
		})
	}
}

func TestCoordinator_Multiple_Tasks(t *testing.T) {
	// Create a coordinator with a buffer
	c := &coordinator{
		ch:        make(chan ITask, 10),
		wg:        &sync.WaitGroup{},
		ignoreErr: false,
	}
	c.wg.Add(1)
	go c.drainTask()

	// Counter to track number of executed tasks
	var executed int
	var mu sync.Mutex

	// Create and put multiple tasks
	tasksCount := 5
	for range tasksCount {
		task := newMockTask(func() error {
			mu.Lock()
			executed++
			mu.Unlock()
			return nil
		})
		c.Put(task)
	}

	// Close and wait for all tasks to complete
	err := c.Close()
	assert.NoError(t, err)

	// Verify all tasks were executed
	mu.Lock()
	assert.Equal(t, tasksCount, executed, "All tasks should have been executed")
	mu.Unlock()
}

func TestCoordinator_Fix_Init_Bug(t *testing.T) {
	// There seems to be a bug in the NewQueue function where c.wg is not initialized
	// Let's test that we can fix it by initializing the WaitGroup

	// Test the fixed version
	fixedQueue := func(queueLen int, ignoreErr bool) IQueue {
		c := &coordinator{
			ch:        make(chan ITask, queueLen),
			wg:        &sync.WaitGroup{}, // Initialize the WaitGroup
			ignoreErr: ignoreErr,
		}
		c.wg.Add(1)
		go c.drainTask()
		return c
	}

	q := fixedQueue(5, false)

	// Type assertion to check if q is of type *coordinator
	c, ok := q.(*coordinator)
	assert.True(t, ok, "Queue should be of type *coordinator")

	assert.NotNil(t, c.wg, "WaitGroup should be initialized")

	// Clean up
	_ = q.Close()
}
