package main

import "testing"

func TestRoundDec(t *testing.T) {
	type args struct {
		v      float64
		places int
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{"", args{1.2345, 2}, 1.23},
		{"", args{-1.2345, 2}, -1.23},
		{"", args{1.23452, 4}, 1.2345},
		{"", args{1.23, 10}, 1.23},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RoundDec(tt.args.v, tt.args.places); got != tt.want {
				t.Errorf("RoundDec() = %v, want %v", got, tt.want)
			}
		})
	}
}
