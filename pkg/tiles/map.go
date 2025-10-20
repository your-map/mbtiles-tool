package tiles

import (
	"bytes"
	"compress/zlib"
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
	OSM Format = "osm"

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

		blobHeaderSize := int64(binary.BigEndian.Uint32(buf.Bytes()))

		buf.Reset()
		if _, err = io.CopyN(buf, file, blobHeaderSize); err != nil {
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

		data := make([]byte, 0)

		switch blob.Data.(type) {
		case *osmpbf.Blob_Raw:
			fmt.Println("\nis row:", blob.GetRaw())

			data = blob.GetRaw()
		case *osmpbf.Blob_ZlibData:
			fmt.Println("\nis zlib: ", blob.GetZlibData())

			r, err := zlib.NewReader(bytes.NewReader(blob.GetZlibData()))
			if err != nil {
				return nil, err
			}

			buf = bytes.NewBuffer(make([]byte, 0, blob.GetRawSize()+bytes.MinRead))
			_, err = buf.ReadFrom(r)
			if err != nil {
				return nil, err
			}
			if buf.Len() != int(blob.GetRawSize()) {
				err = fmt.Errorf("raw blob data size %d but expected %d", buf.Len(), blob.GetRawSize())
				return nil, err
			}

			data = buf.Bytes()
		}

		primitiveBlock := &osmpbf.PrimitiveBlock{}
		if err = proto.Unmarshal(data, primitiveBlock); err != nil {
			return nil, err
		}

		fmt.Println("\nPrimitive block: ", primitiveBlock)

		for _, v := range primitiveBlock.GetPrimitivegroup() {
			fmt.Println("\nChangesets: ", v.Changesets)
			fmt.Println("\nNodes: ", v.Nodes)
			fmt.Println("\nDense: ", v.Dense)
			fmt.Println("\nWays: ", v.Ways)
			fmt.Println("\nRelations: ", v.Relations)
		}

		return NewMap(m.File), nil
	}

	return nil, fmt.Errorf("unknown format: %s", format)
}

func (m *Map) Format() (Format, error) {
	return OSM, nil
}
