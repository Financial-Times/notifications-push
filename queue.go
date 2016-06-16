package main

type item interface{}

type queue interface {
	enqueue(i item)
	dequeue() item
	items() []item
}

type circularBuffer struct {
	buffer []item
}

func NewCircularBuffer(capacity int) queue {
	return &circularBuffer{make([]item, 0, capacity)}
}

func (cb *circularBuffer) enqueue(i item) {
	if cb.isFull() {
		cb.dequeue()
	}
	cb.buffer = append(cb.buffer, i)
}

func (cb *circularBuffer) dequeue() item {
	if len(cb.buffer) > 0 {
		i := cb.buffer[0]
		cb.buffer = cb.buffer[1:]
		return i
	}
	return nil
}

func (cb *circularBuffer) items() []item {
	return cb.buffer
}

func (cb *circularBuffer) isFull() bool {
	return len(cb.buffer) == cap(cb.buffer)
}
