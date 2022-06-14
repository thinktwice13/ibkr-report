package main

import (
	"reflect"
	"testing"
	"time"
)

func Test_amountFromString(t *testing.T) {
	tests := []struct {
		name string
		args string
		want float64
	}{
		{"empty", "", 0},
		{"decimal point", "22.33", 22.33},
		{"decimal point with 000 separator ", "1,222.33", 1222.33},
		{"decimal comma", "22,33", 22.33},
		{"decimal comma with 000 separator", "1.222,33", 1222.33},
		{"with spaces", "1. 222 ,333 .44", 1222333.44},
		{"neg with spaces", "-1. 222 ,333 .44", -1222333.44},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := amountFromString(tt.args); got != tt.want {
				t.Errorf("amountFromString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_formatISIN(t *testing.T) {
	tests := []struct {
		name string
		args string
		want string
	}{
		{"", "", ""},
		{"", "12345678", "12345678"},
		{"", "1234567890123", "1234567890123"},
		{"US", "3456789012", "US345678901"},
		{"US", "IE3456789012", "IE345678901"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatISIN(tt.args); got != tt.want {
				t.Errorf("formatISIN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mapIbkrLine(t *testing.T) {
	type args struct {
		data   []string
		header []string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{
		{"empty header", args{[]string{"v"}, nil}, nil, true},
		{"empty data", args{nil, []string{"v"}}, nil, true},
		{"length mismatch", args{[]string{"v"}, []string{"v", "n"}}, nil, true},
		{"correct", args{[]string{"MySection", "Bar"}, []string{"MySection", "Foo"}}, map[string]string{"Section": "MySection", "Foo": "Bar"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapIbkrLine(tt.args.data, tt.args.header)
			if (err != nil) != tt.wantErr {
				t.Errorf("mapIbkrLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mapIbkrLine() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_symbolFromDescription(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    string
		wantErr bool
	}{
		{"empty input", "", "", true},
		{"no symbol", "(US26924G8134)", "", true},
		{"correct", "AIEQ(US26924G8134)", "AIEQ", false},
		{"correct with space", "AIEQ (US26924G8134)", "AIEQ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := symbolFromDescription(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("symbolFromDescription() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("symbolFromDescription() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_yearFromDate(t *testing.T) {
	tests := []struct {
		name string
		args string
		want int
	}{
		{"", "2018-12-1, 12:34", 2018},
		{"", "ipu7t08go8i", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := yearFromDate(tt.args); got != tt.want {
				t.Errorf("yearFromDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_timeFromExact(t *testing.T) {
	fn := func() *time.Time {
		t := time.Date(2019, 10, 14, 12, 55, 53, 0, time.UTC)
		return &t
	}

	tests := []struct {
		name    string
		args    string
		want    *time.Time
		wantErr bool
	}{
		{"date fails", "2019-10-14", nil, true},
		{"correct", "2019-10-14, 12:55:53", fn(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := timeFromExact(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("timeFromExact() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("timeFromExact() got = %v, want %v", got, tt.want)
			}
		})
	}
}
