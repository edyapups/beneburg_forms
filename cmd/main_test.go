package main

import "testing"

func Test_hideID(t *testing.T) {
	type args struct {
		id int64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "Test 1", args: args{id: 222314}, want: "12245220446411813960"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hideID(tt.args.id); got != tt.want {
				t.Errorf("hideID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_admitID(t *testing.T) {
	type args struct {
		id string
	}
	tests := []struct {
		name    string
		args    args
		want    int64
		wantErr bool
	}{
		{name: "Test 1", args: args{id: "12245220446411813960"}, want: 222314, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := admitID(tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("admitID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("admitID() got = %v, want %v", got, tt.want)
			}
		})
	}
}
