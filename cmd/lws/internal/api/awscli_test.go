package api

import "testing"

func TestParamsNormaliser(t *testing.T) {

	tests := []struct {
		name   string
		desc   string
		input  map[string][]string
		expect map[string]string
	}{
		{
			name:   "EmptyInput",
			desc:   "must return an empty result",
			input:  map[string][]string{},
			expect: map[string]string{},
		},
		{
			name:   "OnlyParams",
			desc:   "has no attributes, only two parameters. Should return flattened parameters",
			input:  map[string][]string{"Param1": {"val1"}, "Param2": {"val2"}},
			expect: map[string]string{"Param1": "val1", "Param2": "val2"},
		},
		{
			name: "AloneAttr",
			desc: "the input contains only one attribute which should be the result",
			input: map[string][]string{
				"Attribute.1.Name":  {"Attr1Name"},
				"Attribute.1.Value": {"Attr1Val"},
			},
			expect: map[string]string{"Attr1Name": "Attr1Val"},
		},
		{
			name:   "InvalidAttr1",
			desc:   "the input has one valid attribute name but no value. Should return none",
			input:  map[string][]string{"Attribute.1.Name": {"Attr1Name"}},
			expect: map[string]string{},
		},
		{
			name: "InvalidAttr2",
			desc: "the input has one valid attribute name but no matching value",
			input: map[string][]string{
				"Attribute.1.Name":  {"Attr1Name"},
				"Attribute.2.Value": {"Attr2Val"},
			},
			expect: map[string]string{},
		},
		{
			name: "MultipleAttr1",
			desc: "the input has more than one valid attributes",
			input: map[string][]string{
				"Attribute.1.Name":  {"Attr1Name"},
				"Attribute.3.Name":  {"Attr3Name"},
				"Attribute.3.Value": {"Attr3Val"},
				"Attribute.1.Value": {"Attr1Val"},
			},
			expect: map[string]string{"Attr1Name": "Attr1Val", "Attr3Name": "Attr3Val"},
		},
		{
			name: "MixedInput",
			desc: "the input has more than one valid attributes and parameters, return flattened",
			input: map[string][]string{
				"Param2":            {"Param2Val"},
				"Attribute.1.Name":  {"Attr1Name"},
				"Attribute.3.Name":  {"Attr3Name"},
				"Attribute.3.Value": {"Attr3Val"},
				"Param1":            {"Param1Val"},
				"Attribute.1.Value": {"Attr1Val"},
			},
			expect: map[string]string{
				"Attr1Name": "Attr1Val",
				"Attr3Name": "Attr3Val",
				"Param2":    "Param2Val",
				"Param1":    "Param1Val",
			},
		},
	}

	areDiff := func(res, expected map[string]string) bool {
		if len(res) != len(expected) {
			return true
		}

		for k, v := range res {
			v2, ok := expected[k]
			if !ok {
				return true
			}
			if v != v2 {
				return true
			}
		}

		return false
	}

	for _, test := range tests {
		res := flattAndParse(test.input)
		if areDiff(res, test.expect) {
			t.Log(test.name, "expects", test.expect, "got", res, "Desc:", test.desc)
			t.Fail()
		}
	}
}
