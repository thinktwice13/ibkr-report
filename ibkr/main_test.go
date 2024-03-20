package ibkr

import (
	"testing"
)

func Test_amountFromString(t *testing.T) {
	tests := []struct {
		in  string
		out float64
	}{
		{"1.23", 1.23},
		{"-79,9 78.978 67", -79978.97867},
		{"-79....97,,,,8.97,8 67", -79978.97867},
	}

	for _, tt := range tests {
		if got := amountFromString(tt.in); got != tt.out {
			t.Errorf("amountFromString(%q) = %v; want %v", tt.in, got, tt.out)
		}
	}
}

func Benchmark_amountFromString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		amountFromString("-79....97,,,,8.97,8 67")
	}
}

func Benchmark_amountFromStringOld(b *testing.B) {
	for i := 0; i < b.N; i++ {
		amountFromStringOld("-79....97,,,,8.97,8 67")
	}
}
