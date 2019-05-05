package params

import (
	"fmt"
	"testing"
)

func TestValUI32(t *testing.T) {

	testsSet := []struct {
		description string
		k           string
		d           uint32
		l           uint32
		u           uint32
		p           map[string]string
		//
		expectedVal uint32
		expectedErr string
	}{
		{
			"Tests default value when k is no in p",
			"key1",
			7,
			0,
			10,
			map[string]string{},
			7,
			"",
		},
		{
			"Tests lower limit",
			"key1",
			7,
			5,
			10,
			map[string]string{"key1": "2"},
			7,
			"invalid value for the parameter key1",
		},
		{
			"Tests upper limit",
			"key1",
			7,
			5,
			10,
			map[string]string{"key1": "11"},
			7,
			"invalid value for the parameter key1",
		},
		{
			"Tests parsing error handling",
			"key1",
			7,
			5,
			10,
			map[string]string{"key1": "abc"},
			7,
			"invalid value for the parameter key1",
		},
	}

	for _, test := range testsSet {
		val, err := ValUI32(test.k, test.d, test.l, test.u, test.p)
		if test.expectedErr != "" || err != nil {
			if test.expectedErr != fmt.Sprintf("%s", err) {
				t.Errorf(
					"Error mismatch. %s. Expects %q, got %q",
					test.description, test.expectedErr, err,
				)
			}
			continue
		}

		if val != test.expectedVal {
			t.Errorf("%s. Expects %d, got %d", test.description, test.expectedVal, val)
		}
	}

}

func TestBool(t *testing.T) {

	testsSet := []struct {
		description string
		k           string
		d           bool
		p           map[string]string
		//
		expectedVal bool
		expectedErr string
	}{
		{
			"Tests default value when k is no in p",
			"key1",
			true,
			map[string]string{},
			true,
			"",
		},
		{
			"Tests parsing error handling",
			"key1",
			true,
			map[string]string{"key1": "abc", "key2": "false"},
			false,
			"invalid value for the parameter key1",
		},
		{
			"Tests getting existing valid k from p",
			"key1",
			true,
			map[string]string{"key1": "true", "key2": "abc"},
			true,
			"",
		},
	}

	for _, test := range testsSet {
		val, err := ValBool(test.k, test.d, test.p)
		if test.expectedErr != "" || err != nil {
			if test.expectedErr != fmt.Sprintf("%s", err) {
				t.Errorf(
					"Error mismatch. %s. Expects %q, got %q",
					test.description, test.expectedErr, err,
				)
			}
			continue
		}

		if val != test.expectedVal {
			t.Errorf("%s. Expects %t, got %t", test.description, test.expectedVal, val)
		}
	}

}
