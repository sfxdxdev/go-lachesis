package common

import (
	"encoding/json"
	"testing"
)

func TestAddressUnmarshalJSON(t *testing.T) {
	var tests = []struct {
		Input     string
		ShouldErr bool
	}{
		{"", true},
		{`""`, true},
		{`"0x"`, true},
		{`"0x00"`, true},
		{`"0xG000000000000000000000000000000000000000000000000000000000000000"`, true},
		{`"0x0000000000000000000000000000000000000000000000000000000000000000"`, false},
		{`"0x0000000000000000000000000000000000000000000000000000000000000010"`, false},
	}
	for i, test := range tests {
		var v Address
		err := json.Unmarshal([]byte(test.Input), &v)
		if err != nil && !test.ShouldErr {
			t.Errorf("test #%d: unexpected error: %v", i, err)
		}
		if err == nil {
			if test.ShouldErr {
				t.Errorf("test #%d: expected error, got none", i)
			}
		}
	}
}
