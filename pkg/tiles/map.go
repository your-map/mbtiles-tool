package tiles

import (
	"errors"
	"fmt"
	"os"
)

var (
	MBT Format = "mbt"

	ErrReadFile = errors.New("error reading file")
)

type Format string

type Map struct {
	File string
}

func NewMap(file string) *Map {
	return &Map{
		File: file,
	}
}

func (m *Map) Convert(format Format) (*Map, error) {
	switch format {
	case MBT:
		data, err := os.ReadFile(m.File)
		if err != nil {
			return nil, ErrReadFile
		}

		fmt.Println(string(data))

		return NewMap(m.File), nil
	}

	return nil, fmt.Errorf("unknown format: %s", format)
}
