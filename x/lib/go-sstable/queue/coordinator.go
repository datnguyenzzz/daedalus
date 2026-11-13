package queue

import "sync"

type IQueue interface {
	// Put publisher when use this function will be blocked until
	// this function is finished
	Put(ITask)
	Close() error
}

type coordinator struct {
	ch        chan ITask
	wg        *sync.WaitGroup
	err       error
	closed    bool
	ignoreErr bool
}

func (c *coordinator) drainTask() {
	defer c.wg.Done()
	for task := range c.ch {
		task.OnHold()
		if c.err == nil || c.ignoreErr {
			c.err = task.Execute()
		}

		task.Release()
	}
}

func (c *coordinator) Put(task ITask) {
	c.ch <- task
}

func (c *coordinator) Close() error {
	if c.closed {
		return c.err
	}
	close(c.ch)
	// Ensure all tasks are running to finish wholly
	c.wg.Wait()
	c.closed = true
	return c.err
}

func NewQueue(queueLen int, ignoreErr bool) IQueue {
	c := &coordinator{
		ch:        make(chan ITask, queueLen),
		ignoreErr: ignoreErr,
		wg:        &sync.WaitGroup{},
	}
	c.wg.Add(1)
	go c.drainTask()
	return c
}
