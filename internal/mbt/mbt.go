package mbt

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"

	"github.com/your-map/mbtiles-tool/internal/osm/proto"
)

type MBT struct {
	db           *sql.DB
	layerStats   map[string]*LayerStat
	vectorLayers map[string]*VectorLayer
	fieldTypes   map[string]map[string]string
}

func NewMBT() (*MBT, error) {
	db, err := sql.Open("sqlite3", "test.mbtiles")
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
		db:           db,
		layerStats:   make(map[string]*LayerStat),
		vectorLayers: make(map[string]*VectorLayer),
		fieldTypes:   make(map[string]map[string]string),
	}, nil
}

func (m *MBT) WriteMetaData(metaData *proto.HeaderBlock) error {
	stmt, err := m.db.Prepare("INSERT OR REPLACE INTO metadata (name, value) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer func(stmt *sql.Stmt) {
		err = stmt.Close()
		if err != nil {
			log.Fatal(err)
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

	if metaData.Writingprogram != nil {
		metadataFields["writingprogram"] = *metaData.Writingprogram
	}

	if metaData.Source != nil {
		metadataFields["source"] = *metaData.Source
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

func (m *MBT) WriteBlockData(data *proto.PrimitiveBlock) error {
	stringTable := data.GetStringtable().GetS()

	// Подготовка statement для вставки тайлов
	stmt, err := m.db.Prepare("INSERT OR REPLACE INTO tiles (zoom_level, tile_column, tile_row, tile_data) VALUES (?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare tiles statement: %w", err)
	}
	defer stmt.Close()

	// Собираем все точки для упрощенной визуализации
	var points []struct {
		lat, lon float64
		tags     map[string]string
	}

	for _, group := range data.GetPrimitivegroup() {
		// Обработка обычных нод
		for _, node := range group.GetNodes() {
			m.analyzePrimitive("nodes", node.GetKeys(), node.GetVals(), stringTable, "Point")

			lat, lon := m.decodeCoordinates(node.GetLat(), node.GetLon(), data)
			tags := m.extractTags(node.GetKeys(), node.GetVals(), stringTable)

			points = append(points, struct {
				lat, lon float64
				tags     map[string]string
			}{lat, lon, tags})
		}

		// Обработка dense nodes
		if dense := group.GetDense(); dense != nil {
			m.analyzeDenseNodes(dense, stringTable)

			densePoints := m.extractDenseNodes(dense, data, stringTable)
			points = append(points, densePoints...)
		}

		// Анализ ways и relations (пока без генерации тайлов)
		for _, way := range group.GetWays() {
			m.analyzePrimitive("ways", way.GetKeys(), way.GetVals(), stringTable, "LineString")
		}

		for _, relation := range group.GetRelations() {
			m.analyzePrimitive("relations", relation.GetKeys(), relation.GetVals(), stringTable, "GeometryCollection")
		}
	}

	// Создаем простые тайлы с точками
	return m.createSimpleTiles(points, stmt)
}

func (m *MBT) Close() error {
	return m.db.Close()
}
