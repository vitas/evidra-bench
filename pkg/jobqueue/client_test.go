package jobqueue

import (
	"testing"
)

func TestNamespaceSlot_RoundRobin(t *testing.T) {
	t.Parallel()
	scenarios := []string{"a", "b", "c", "d", "e"}
	parallel := 3
	expected := []int{0, 1, 2, 0, 1}
	for i, s := range scenarios {
		workerID := i % parallel
		if workerID != expected[i] {
			t.Errorf("scenario %s: expected worker %d, got %d", s, expected[i], workerID)
		}
	}
}
