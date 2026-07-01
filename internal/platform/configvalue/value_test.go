package configvalue

import "testing"

func TestBoolDefaultParsesCommonValues(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		fallback bool
		want     bool
	}{
		{name: "nil uses fallback true", value: nil, fallback: true, want: true},
		{name: "nil uses fallback false", value: nil, fallback: false, want: false},
		{name: "bool", value: true, fallback: false, want: true},
		{name: "true string", value: "true", fallback: false, want: true},
		{name: "one string", value: "1", fallback: false, want: true},
		{name: "yes string", value: "yes", fallback: false, want: true},
		{name: "on string", value: "on", fallback: false, want: true},
		{name: "false string", value: "false", fallback: true, want: false},
		{name: "empty string uses fallback", value: " ", fallback: true, want: true},
		{name: "unknown type uses fallback", value: 1, fallback: false, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BoolDefault(tc.value, tc.fallback); got != tc.want {
				t.Fatalf("BoolDefault(%#v, %v) = %v, want %v", tc.value, tc.fallback, got, tc.want)
			}
		})
	}
}
