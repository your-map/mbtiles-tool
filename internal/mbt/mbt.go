package mbt

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/your-map/mbtiles-tool/internal/osm/proto"
)

type MBT struct {
	db *sql.DB
}

func NewMBT() (*MBT, error) {
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

	return nil
}

func (m *MBT) Close() error {
	return m.db.Close()
}
