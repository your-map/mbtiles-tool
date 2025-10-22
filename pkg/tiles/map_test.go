package tiles

import (
	"reflect"
	"testing"
)

func TestMap_Convert(t *testing.T) {
	type fields struct {
		File string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Map
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Map{
				File: tt.fields.File,
			}
			got, err := m.Convert()
			if (err != nil) != tt.wantErr {
				t.Errorf("Convert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Convert() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMap_Format(t *testing.T) {
	type fields struct {
		File string
	}
	tests := []struct {
		name    string
		fields  fields
		want    Format
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Map{
				File: tt.fields.File,
			}
			got, err := m.Format()
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Format() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewMap(t *testing.T) {
	type args struct {
		file string
	}
	tests := []struct {
		name string
		args args
		want *Map
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMap(tt.args.file); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
