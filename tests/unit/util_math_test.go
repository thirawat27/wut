package unit

import (
	"testing"

	"wut/internal/util"
)

func TestMin(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"first smaller", 1, 2, 1},
		{"second smaller", 5, 3, 3},
		{"equal", 4, 4, 4},
		{"negative numbers", -5, -3, -5},
		{"mixed signs", -1, 1, -1},
		{"zero", 0, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.Min(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Min(%d, %d) = %d; want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}
