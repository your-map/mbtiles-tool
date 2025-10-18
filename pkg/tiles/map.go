package tiles

import "fmt"

type Format string

var (
	MBT Format = "mbt"
)

type Map struct {
	File string
}

func NewMap(file string) *Map {
	return &Map{
		File: file,
	}
}

// @todo #3 add base logic
func (m *Map) Convert(format Format) (*Map, error) {
	switch format {
	case MBT:
		// @todo add use converter for map
		return NewMap(m.File), nil
	}

	return nil, fmt.Errorf("unknown format: %s", format)
}
