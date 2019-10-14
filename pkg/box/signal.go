package box

import "io"

const signalStr string = "#booted#"

type signal struct {
	signaled bool
	i        int
	source   io.Writer
	c        chan struct{}
}

func (s *signal) isIn(src []byte) bool {
	if s.signaled {
		return true
	}

	for _, b := range src {
		if b != signalStr[s.i] {
			s.i = 0
			continue
		}

		s.i++

		if s.i == len(signalStr) {
			s.signaled = true
			return true
		}
	}

	return false
}

func (s *signal) Write(p []byte) (n int, err error) {
	if !s.signaled && s.isIn(p) {
		s.c <- struct{}{}
	}

	return s.source.Write(p)
}
