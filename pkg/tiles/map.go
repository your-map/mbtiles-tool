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
	const maxBlobSize = 64 * 1024 * 1024

	switch format {
	case MBT:
		buf := new(bytes.Buffer)

		file, err := os.Open(m.File)
		if err != nil {
			return nil, ErrReadFile
		}
		defer file.Close()

		for {
			buf.Reset()

			if _, err = io.CopyN(buf, file, 4); err != nil {
				if err == io.EOF {
					break
				}
				return nil, ErrReadFile
			}

			blobHeaderSize := int64(binary.BigEndian.Uint32(buf.Bytes()))

			buf.Reset()
			if _, err = io.CopyN(buf, file, blobHeaderSize); err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}

			blobHeader := new(osmpbf.BlobHeader)
			if err := proto.Unmarshal(buf.Bytes(), blobHeader); err != nil {
				return nil, err
			}

			if blobHeader.GetDatasize() > maxBlobSize {
				return nil, fmt.Errorf("blob size too large: %d", blobHeader.GetDatasize())
			}

			buf.Reset()
			if _, err = io.CopyN(buf, file, int64(blobHeader.GetDatasize())); err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}

			blob := new(osmpbf.Blob)
			if err = proto.Unmarshal(buf.Bytes(), blob); err != nil {
				return nil, err
			}

			data := make([]byte, 0)

			switch blob.Data.(type) {
			case *osmpbf.Blob_Raw:
				data = blob.GetRaw()
			case *osmpbf.Blob_ZlibData:
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
			default:
				fmt.Println("\nunknown blob data type")
			}

			switch blobHeader.GetType() {
			case "OSMHeader":
				headerBlock := &osmpbf.HeaderBlock{}
				if err = proto.Unmarshal(data, headerBlock); err != nil {
					return nil, err
				}

				fmt.Println("\nOSMHeader: ", headerBlock)
			case "OSMData":
				primitiveBlock := &osmpbf.PrimitiveBlock{}
				if err = proto.Unmarshal(data, primitiveBlock); err != nil {
					return nil, err
				}

				fmt.Printf("\n\nPrimitive block string table size: %d", len(primitiveBlock.GetStringtable().GetS()))

				for _, group := range primitiveBlock.GetPrimitivegroup() {
					if dense := group.GetDense(); dense != nil {
						fmt.Printf("\n\nIsset Dense")
						fmt.Printf("\nDense LAT: %v", dense.Lat)
						fmt.Printf("\nDense LON: %v", dense.Lon)
					}

					if nodes := group.GetNodes(); nodes != nil {
						fmt.Printf("\n\nIsset Nodes")
						fmt.Printf("\nNodes: %v", nodes)
					}

					if ways := group.GetWays(); ways != nil {
						fmt.Printf("\n\n Isset Ways")
						fmt.Printf("\nWays: %v", ways)
					}

					if relations := group.GetRelations(); relations != nil {
						fmt.Printf("\n\nIsset Relations")
						fmt.Printf("\nRelations : %v", relations)
					}
				}

			default:
				fmt.Printf("Unknown block type: %s\n", blobHeader.GetType())
			}
		}
	}

	return NewMap(m.File), nil
}

func (m *Map) Format() (Format, error) {
	return OSM, nil
}
