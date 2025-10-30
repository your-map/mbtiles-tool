package mbt

import (
	"database/sql"
	"fmt"

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

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS metadata (
			name  text,
			value text
		);
		
		CREATE TABLE IF NOT EXISTS tiles (
			zoom_level  integer,
			tile_column integer,
			tile_row    integer,
			tile_data   blob
		);
		
		CREATE UNIQUE INDEX IF NOT EXISTS tile_index 
		ON tiles (zoom_level, tile_column, tile_row);
	`)
	if err != nil {
		return nil, err
	}

	return &MBT{
		db: db,
	}, nil
}

func (m *MBT) WriteBlockData(data *proto.PrimitiveBlock) error {
	fmt.Println(data.GetStringtable())
	return nil
}

func (m *MBT) WriteMetaData(metaData *proto.HeaderBlock) error {
	stmt, err := m.db.Prepare("INSERT OR REPLACE INTO metadata (name, value) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			panic(err)
		}
	}(stmt)

	metadataFields := map[string]string{
		"name":    "OSM Data",
		"version": "1.3",
		"format":  "pbf",
		"type":    "overlay",
		"minzoom": "0",
		"maxzoom": "14",
	}

	if len(metaData.RequiredFeatures) > 0 {
		metadataFields["name"] = metaData.RequiredFeatures[0]
	}

	if metaData.Bbox != nil {
		grid := NewGrid(metaData.Bbox)
		bounds, center := grid.Execute()

		metadataFields["bounds"] = bounds
		metadataFields["center"] = center
	}

	for name, value := range metadataFields {
		_, err = stmt.Exec(name, value)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *MBT) Close() error {
	return m.db.Close()
}
