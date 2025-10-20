package tiles

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/your-map/mbtiles-tool/internal/osmpbf"
	"google.golang.org/protobuf/proto"
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
		file, err := os.Open(m.File)
		if err != nil {
			return nil, ErrReadFile
		}
		defer file.Close()

		buf := new(bytes.Buffer)

		buf.Reset()
		if _, err = io.CopyN(buf, file, 4); err != nil {
			return nil, ErrReadFile
		}

		buf.Reset()
		if _, err = io.CopyN(buf, file, int64(binary.BigEndian.Uint32(buf.Bytes()))); err != nil {
			return nil, err
		}

		blobHeader := new(osmpbf.BlobHeader)
		if err := proto.Unmarshal(buf.Bytes(), blobHeader); err != nil {
			return nil, err
		}

		buf.Reset()
		if _, err = io.CopyN(buf, file, int64(blobHeader.GetDatasize())); err != nil {
			return nil, err
		}

		blob := new(osmpbf.Blob)
		if err = proto.Unmarshal(buf.Bytes(), blob); err != nil {
			return nil, err
		}

		fmt.Println("Blob header: ", blobHeader)
		fmt.Println("blob: ", blob)

		return NewMap(m.File), nil
	}

	return nil, fmt.Errorf("unknown format: %s", format)
}
