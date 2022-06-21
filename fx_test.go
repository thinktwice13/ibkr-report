package main

import (
	"testing"
	"time"
)

func Test_fxDateFromYear(t *testing.T) {
	type args struct {
		y int
	}
	tests := []struct {
		name string
		args int
		want string
	}{
		{"2020", 2020, "2020-12-31"},
		{"current", time.Now().Year(), time.Now().Format("2006-01-02")},
		{"current", time.Now().Year() + 2, time.Now().Format("2006-01-02")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fxDateFromYear(tt.args); got != tt.want {
				t.Errorf("fxDateFromYear() = %v, want %v", got, tt.want)
			}
		})
	}
}
