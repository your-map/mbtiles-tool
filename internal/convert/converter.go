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
	newOSM := osm.NewOSM(c.File)

	newMBT, err := mbt.NewMBT()
	if err != nil {
		return err
	}
	defer func(newMBT *mbt.MBT) {
		err = newMBT.Close()
		if err != nil {
			panic(err)
		}
	}(newMBT)

	dataChan, err := newOSM.Read()
	if err != nil {
		return err
	}

	for data := range dataChan {
		if data.Header != nil {
			err = newMBT.WriteMetaData(data.Header)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
