package box

import "testing"

// Gets the signal in just one go
func TestSignalAtOnce(t *testing.T) {
	s := &signal{}
	if !s.isIn([]byte("no\nise" + signalStr + "more noi\nse")) {
		t.Errorf("should spot the signal in one go")
	}
}

// Checks if the start offset is correct
func TestStartOffset(t *testing.T) {
	s := &signal{}

	found := s.isIn([]byte("booted#"))
	found = found || s.isIn([]byte("#"))

	if found {
		t.Errorf("start offset seems to be wrong")
	}
}

// Checks if reset is being done after a mismatch on characters
func TestReset(t *testing.T) {
	s := &signal{}

	found := s.isIn([]byte("abc#"))
	found = found || s.isIn([]byte("z"))
	found = found || s.isIn([]byte("booted#def"))

	if found {
		t.Errorf("didn't reset signal's cursor?")
	}
}

// Checks if a splited signal can be spotted
func TestSplitedSignal(t *testing.T) {
	s := &signal{}

	counter := 0
	if s.isIn([]byte("abc#")) {
		counter++
	}
	if s.isIn([]byte("boo")) {
		counter++
	}
	if s.isIn([]byte("ted#def")) {
		counter++
	}

	if counter != 1 {
		t.Errorf("couldn't spot a splited signal")
	}
}
