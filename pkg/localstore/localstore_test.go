package localstore

import "testing"

func TestOpenCreatesLocalStore(t *testing.T) {
	t.Parallel()

	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()
}
