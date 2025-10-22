package osm

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"

	osmp "github.com/your-map/mbtiles-tool/internal/osm/proto"

	"google.golang.org/protobuf/proto"
)

const (
	maxBlobSize = 64 * 1024 * 1024
)

type OSM struct {
	File io.Reader
}

func NewOSM(r io.Reader) *OSM {
	return &OSM{File: r}
}

func (o *OSM) Read() error {
	buf := new(bytes.Buffer)

	for {
		buf.Reset()
		if _, err := io.CopyN(buf, o.File, 4); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		blobHeaderSize := int64(binary.BigEndian.Uint32(buf.Bytes()))

		buf.Reset()
		if _, err := io.CopyN(buf, o.File, blobHeaderSize); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		blobHeader := new(osmp.BlobHeader)
		if err := proto.Unmarshal(buf.Bytes(), blobHeader); err != nil {
			return err
		}

		if blobHeader.GetDatasize() > maxBlobSize {
			return fmt.Errorf("blob size too large: %d", blobHeader.GetDatasize())
		}

		buf.Reset()
		if _, err := io.CopyN(buf, o.File, int64(blobHeader.GetDatasize())); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		blob := new(osmp.Blob)
		if err := proto.Unmarshal(buf.Bytes(), blob); err != nil {
			return err
		}

		data := make([]byte, 0)

		switch blob.Data.(type) {
		case *osmp.Blob_Raw:
			data = blob.GetRaw()
		case *osmp.Blob_ZlibData:
			r, err := zlib.NewReader(bytes.NewReader(blob.GetZlibData()))
			if err != nil {
				return err
			}

			buf = bytes.NewBuffer(make([]byte, 0, blob.GetRawSize()+bytes.MinRead))
			if _, err = buf.ReadFrom(r); err != nil {
				return err
			}

			if buf.Len() != int(blob.GetRawSize()) {
				return fmt.Errorf("raw blob data size %d but expected %d", buf.Len(), blob.GetRawSize())
			}

			data = buf.Bytes()
		default:
			fmt.Println("\nunknown blob data type")
		}

		switch blobHeader.GetType() {
		case "OSMHeader":
			headerBlock := &osmp.HeaderBlock{}
			if err := proto.Unmarshal(data, headerBlock); err != nil {
				return err
			}

			fmt.Println("\nOSMHeader: ", headerBlock)
		case "OSMData":
			primitiveBlock := &osmp.PrimitiveBlock{}
			if err := proto.Unmarshal(data, primitiveBlock); err != nil {
				return err
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

	return nil
}
