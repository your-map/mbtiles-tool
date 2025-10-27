package convert

import (
	"io"

	"github.com/your-map/mbtiles-tool/internal/mbt"
	"github.com/your-map/mbtiles-tool/internal/osm"
)

type Converter struct {
	File io.Reader
}

func NewConverter(r io.Reader) *Converter {
	return &Converter{File: r}
}

func (c *Converter) OsmConvert() error {
	om := osm.NewOSM(c.File)

	if err := om.Read(); err != nil {
		return err
	}

	err := mbt.Init()
	if err != nil {
		return err
	}

	return nil
}
