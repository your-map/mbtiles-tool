package tiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/your-map/mbtiles-tool/internal/osm"
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
	defer func() {
		if err = file.Close(); err != nil {
			panic(err)
		}
	}()

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
	filename := filepath.Base(m.File)

	for format, ext := range FormatFileExt {
		if strings.HasSuffix(filename, ext) {
			return format, nil
		}
	}

	return Unknown, errors.New("unknown format: " + filename)
}
