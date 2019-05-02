package params

import (
	"fmt"
	"strconv"
)

// ValString returns the parameter with the given key, or an empty string if it doesn't exists.
func ValString(k string, p map[string]string) (v string) {

	v, _ = p[k]
	return
}

// ValBool returns a boolean parameter if present, d otherwise.
func ValBool(k string, d bool, p map[string]string) (v bool, err error) {

	bStr, ok := p[k]
	if !ok {
		v = d
		return
	}

	tErr := fmt.Errorf("invalid value for the parameter %s", k)
	b, err := strconv.ParseBool(bStr)
	if err != nil {
		err = tErr
		return
	}

	v = b

	return
}

// ValUI32 returns a uint32 parameter if present, d otherwise, checking lower and upper bounds.
func ValUI32(k string, d, l, u uint32, p map[string]string) (v uint32, err error) {

	s, ok := p[k]
	if !ok {
		v = d
		return
	}

	tErr := fmt.Errorf("invalid value for the parameter %s", k)
	v64, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		err = tErr
		return
	}

	v = uint32(v64)
	// check bounds
	if v < l || v > u {
		err = tErr
		return
	}

	return
}
