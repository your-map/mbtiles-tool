package mbt

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/your-map/mbtiles-tool/internal/osm/proto"
)

type MBT struct {
	db *sql.DB
}

func New() (*MBT, error) {
	db, err := sql.Open("sqlite3", "test.db")
	if err != nil {
		return nil, err
	}

	query, err := os.ReadFile("internal/mbt/schema.sql")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(string(query))
	if err != nil {
		return nil, err
	}

	return &MBT{
		db: db,
	}, nil
}

func (m *MBT) WriteBlockData(data *proto.PrimitiveBlock) error {
	fmt.Println(data.GetStringtable().String())
	return nil
}

func (m *MBT) WriteMetaData(metaData *proto.HeaderBlock) error {
	stmt, err := m.db.Prepare("INSERT INTO metadata (name, value) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			panic(err)
		}
	}(stmt)

	if len(metaData.RequiredFeatures) > 0 {
		_, err = stmt.Exec("name", metaData.RequiredFeatures[0])
		if err != nil {
			return err
		}
	}

	_, err = stmt.Exec("version", "1.3")
	if err != nil {
		return err
	}

	_, err = stmt.Exec("format", "pbf")
	if err != nil {
		return err
	}

	_, err = stmt.Exec("type", "overlay")
	if err != nil {
		return err
	}

	bounds, center, err := executeGridMap(metaData.Bbox)
	if err != nil {
		return err
	}

	_, err = stmt.Exec("bounds", bounds)
	if err != nil {
		return err
	}

	_, err = stmt.Exec("center", center)
	if err != nil {
		return err
	}

	_, err = stmt.Exec("minzoom", "0")
	if err != nil {
		return err
	}

	_, err = stmt.Exec("maxzoom", "14")
	if err != nil {
		return err
	}

	return nil
}

func executeGridMap(bbox *proto.HeaderBBox) (string, string, error) {
	if bbox == nil {
		return "", "", errors.New("empty bbox")
	}

	left := float64(bbox.GetLeft()) / 1e9
	right := float64(bbox.GetRight()) / 1e9
	top := float64(bbox.GetTop()) / 1e9
	bottom := float64(bbox.GetBottom()) / 1e9

	centerLon := (left + right) / 2
	centerLat := (bottom + top) / 2

	bounds := fmt.Sprintf("%f,%f,%f,%f", left, bottom, right, top)
	center := fmt.Sprintf("%f,%f,%d", centerLon, centerLat, 10)

	return bounds, center, nil
}

func (m *MBT) Close() error {
	return m.db.Close()
}
