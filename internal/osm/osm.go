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
	maxBlobSize  = 64 * 1024 * 1024
	sizeReadData = 4
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
		headerSize, err := o.headerSize(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		header, err := o.header(buf, headerSize)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		blob, err := o.blob(buf, header)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		data, err := o.data(buf, blob)
		if err != nil {
			return err
		}

		err = o.unmarshalData(data, header)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *OSM) unmarshalData(data []byte, header *osmp.BlobHeader) error {
	switch HeaderType(header.GetType()) {
	case Header:
		headerBlock := &osmp.HeaderBlock{}
		if err := proto.Unmarshal(data, headerBlock); err != nil {
			return err
		}

		fmt.Println("header block: ", headerBlock)
	case Data:
		primitiveBlock := &osmp.PrimitiveBlock{}
		if err := proto.Unmarshal(data, primitiveBlock); err != nil {
			return err
		}

		//for _, group := range primitiveBlock.GetPrimitivegroup() {
		//	if dense := group.GetDense(); dense != nil {
		//		fmt.Printf("\n\nIsset Dense")
		//		fmt.Printf("\nDense LAT: %v", dense.Lat)
		//		fmt.Printf("\nDense LON: %v", dense.Lon)
		//	}
		//
		//	if nodes := group.GetNodes(); nodes != nil {
		//		fmt.Printf("\n\nIsset Nodes")
		//		fmt.Printf("\nNodes: %v", nodes)
		//	}
		//
		//	if ways := group.GetWays(); ways != nil {
		//		fmt.Printf("\n\n Isset Ways")
		//		fmt.Printf("\nWays: %v", ways)
		//	}
		//
		//	if relations := group.GetRelations(); relations != nil {
		//		fmt.Printf("\n\nIsset Relations")
		//		fmt.Printf("\nRelations : %v", relations)
		//	}
		//}
	default:
		return fmt.Errorf("unknown OSM type: %s", header.GetType())
	}

	return nil
}

func (o *OSM) data(buf *bytes.Buffer, blob *osmp.Blob) ([]byte, error) {
	data := make([]byte, 0)

	switch blob.Data.(type) {
	case *osmp.Blob_Raw:
		data = blob.GetRaw()
	case *osmp.Blob_ZlibData:
		r, err := zlib.NewReader(bytes.NewReader(blob.GetZlibData()))
		if err != nil {
			return nil, err
		}
		buf.Reset()

		newBuf := bytes.NewBuffer(make([]byte, 0, blob.GetRawSize()+bytes.MinRead))
		if _, err = newBuf.ReadFrom(r); err != nil {
			return nil, err
		}

		if newBuf.Len() != int(blob.GetRawSize()) {
			return nil, fmt.Errorf("raw blob data size %d but expected %d", newBuf.Len(), blob.GetRawSize())
		}

		data = newBuf.Bytes()
	}

	return data, nil
}

func (o *OSM) blob(buf *bytes.Buffer, header *osmp.BlobHeader) (*osmp.Blob, error) {
	buf.Reset()
	if _, err := io.CopyN(buf, o.File, int64(header.GetDatasize())); err != nil {
		return nil, err
	}

	blob := new(osmp.Blob)
	if err := proto.Unmarshal(buf.Bytes(), blob); err != nil {
		return nil, err
	}

	return blob, nil
}

func (o *OSM) header(buf *bytes.Buffer, headerSize int64) (*osmp.BlobHeader, error) {
	buf.Reset()
	if _, err := io.CopyN(buf, o.File, headerSize); err != nil {
		return nil, err
	}

	header := new(osmp.BlobHeader)
	if err := proto.Unmarshal(buf.Bytes(), header); err != nil {
		return nil, err
	}

	if header.GetDatasize() > maxBlobSize {
		return nil, fmt.Errorf("blob size too large: %d", header.GetDatasize())
	}

	return header, nil
}

func (o *OSM) headerSize(buf *bytes.Buffer) (int64, error) {
	buf.Reset()
	if _, err := io.CopyN(buf, o.File, sizeReadData); err != nil {
		return 0, err
	}

	return int64(binary.BigEndian.Uint32(buf.Bytes())), nil
}
