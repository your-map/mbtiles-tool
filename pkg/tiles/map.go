package tiles

import (
	"errors"
	"os"

	"github.com/your-map/mbtiles-tool/internal/osm"
)

type Format string

var (
	MBT Format = "mbt"
	OSM Format = "osm"
)

type Map struct {
	File string
}

func NewMap(file string) *Map {
	return &Map{
		File: file,
	}
}

func (m *Map) Convert() (*Map, error) {
	format, err := m.Format()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(m.File)
	if err != nil {
		return nil, errors.New("error reading file")
	}
	defer file.Close()

	switch format {
	case OSM:
		om := osm.NewOSM(file)

		if err = om.Read(); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unknown format for convert")
	}

	return NewMap(m.File), nil
}

func (m *Map) Format() (Format, error) {
	return OSM, nil
}
