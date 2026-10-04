//go:build test

package nilcheck

import "testing"

func TestIsNil(t *testing.T) {
	var (
		nilPointer *int
		nilFunc    func()
		nilMap     map[string]int
		nilSlice   []int
		nilChan    chan int
		value      = 1
	)

	cases := []struct {
		name string
		v    any
		want bool
	}{
		{"nil interface", nil, true},
		{"nil pointer", nilPointer, true},
		{"non-nil pointer", &value, false},
		{"nil func", nilFunc, true},
		{"non-nil func", func() {}, false},
		{"nil map", nilMap, true},
		{"non-nil map", map[string]int{}, false},
		{"nil slice", nilSlice, true},
		{"non-nil empty slice", []int{}, false},
		{"nil chan", nilChan, true},
		{"non-nil chan", make(chan int), false},
		{"zero int", 0, false},
		{"zero struct", struct{}{}, false},
		{"empty string", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNil(tc.v); got != tc.want {
				t.Errorf("IsNil(%#v) = %v, want %v", tc.v, got, tc.want)
			}
		})
	}
}
