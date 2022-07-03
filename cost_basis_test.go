package main

import (
	"reflect"
	"testing"
)

func Test_fifo_Sell(t *testing.T) {
	type fields struct {
		data []Cost
	}
	type args struct {
		qty float64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []Cost
		data   []Cost
	}{
		{"empty holdings should return zero costs basis", fields{}, args{qty: -1}, nil, nil},
		{"selling higher qty than next lot with only one lot", fields{[]Cost{{Quantity: 20}}}, args{qty: -25}, nil, nil},
		{"sell exact match qty for next lot", fields{[]Cost{{Quantity: 20}}}, args{qty: -20}, []Cost{{Quantity: 20}}, nil},
		{"selling lower qty", fields{[]Cost{{Quantity: 20}}}, args{qty: -5}, []Cost{{Quantity: 5}}, []Cost{{Quantity: 15}}},
		{"selling higher qty than next lot", fields{[]Cost{{Quantity: 20}, {Quantity: 10}}}, args{qty: -25}, []Cost{{Quantity: 20}, {Quantity: 5}}, []Cost{{Quantity: 5}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fifo{
				data: tt.fields.data,
			}
			got := f.Sell(tt.args.qty)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sell() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(f.data, tt.data) {
				t.Errorf("Sell() = %v, want %v", f.data, tt.data)
			}
		})
	}
}
