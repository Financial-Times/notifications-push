package main

import "testing"

func TestNewCircularBuffer(t *testing.T) {
	maxSize := 5
	cb := NewCircularBuffer(maxSize)
	items := cb.items()
	if len(items) != 0 {
		t.Errorf("Expected empty buffer. Actual: [%d]", len(items))
	}
	if cap(items) != maxSize {
		t.Errorf("Expected buffer with capacity: [%d]. Actual: [%d]", maxSize, cap(items))
	}
}

func TestEnqueue_EnqueuingSizePlusOneNrOfItems_CapacityDoesNotChange(t *testing.T) {
	cb := NewCircularBuffer(2)
	cb.enqueue(1)
	cb.enqueue(2)
	cb.enqueue(3)

	if cap(cb.items()) != 2 {
		t.Errorf("Expected capacity to not change. Actual: [%d]", 2)
	}
}

func TestEnqueue_EnqueuingSizePlusOneNrOfItems_FirstItemInsertedIsDequeued(t *testing.T) {
	cb := NewCircularBuffer(2)
	cb.enqueue(1)
	cb.enqueue(2)
	cb.enqueue(3)

	for _, i := range cb.items() {
		if i == 1 {
			t.Errorf("Expected item [%d] to be dequeued. Actual items: [%v]", 1, cb.items())
		}
	}
}

func TestDequeue_ReadingFromEmptyCircularBuffer_ResultIsNil(t *testing.T) {
	cb := NewCircularBuffer(2)

	if cb.dequeue() != nil {
		t.Errorf("Expected getting [nil] from dequeueing an empty list.")
	}
}

func TestDequeue_EnqueuingSizePlusTwoNrOfItems_DequeuingOrderIsPreserved(t *testing.T) {
	cb := NewCircularBuffer(2)
	cb.enqueue(1)
	cb.enqueue(2)
	cb.enqueue(3)
	cb.enqueue(4)

	third := cb.dequeue()
	fourth := cb.dequeue()

	if third != 3 || fourth != 4 {
		t.Errorf("Dequeued elements are out of order.")
	}
}
