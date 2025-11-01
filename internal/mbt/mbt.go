package mbt

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/your-map/mbtiles-tool/internal/osm/proto"
)

type VectorLayer struct {
	ID          string            `json:"id"`
	Description string            `json:"description,omitempty"`
	MinZoom     int               `json:"minzoom"`
	MaxZoom     int               `json:"maxzoom"`
	Fields      map[string]string `json:"fields"`
}

type TileStats struct {
	LayerCount int         `json:"layerCount"`
	Layers     []LayerStat `json:"layers"`
}

type LayerStat struct {
	Layer          string      `json:"layer"`
	Count          int         `json:"count"`
	Geometry       string      `json:"geometry"`
	AttributeCount int         `json:"attributeCount"`
	Attributes     []Attribute `json:"attributes"`
}

type Attribute struct {
	Attribute string      `json:"attribute"`
	Count     int         `json:"count"`
	Type      string      `json:"type"`
	Values    interface{} `json:"values,omitempty"`
	Min       interface{} `json:"min,omitempty"`
	Max       interface{} `json:"max,omitempty"`
}

type MBT struct {
	db           *sql.DB
	layerStats   map[string]*LayerStat
	vectorLayers map[string]*VectorLayer
	fieldTypes   map[string]map[string]string
}

func NewMBT() (*MBT, error) {
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
		panic(err)
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

	for _, group := range data.GetPrimitivegroup() {
		for _, node := range group.GetNodes() {
			m.analyzePrimitive("nodes", node.GetKeys(), node.GetVals(), stringTable, "Point")
		}

		if dense := group.GetDense(); dense != nil {
			m.analyzeDenseNodes(dense, stringTable)
		}

		for _, way := range group.GetWays() {
			m.analyzePrimitive("ways", way.GetKeys(), way.GetVals(), stringTable, "LineString")
		}

		for _, relation := range group.GetRelations() {
			m.analyzePrimitive("relations", relation.GetKeys(), relation.GetVals(), stringTable, "GeometryCollection")
		}
	}

	return nil
}

func (m *MBT) analyzePrimitive(layer string, keys, vals []uint32, stringTable [][]byte, geometry string) {
	if _, exists := m.layerStats[layer]; !exists {
		m.layerStats[layer] = &LayerStat{
			Layer:    layer,
			Geometry: geometry,
			Count:    0,
		}
		m.fieldTypes[layer] = make(map[string]string)
	}

	m.layerStats[layer].Count++

	for i := 0; i < len(keys) && i < len(vals); i++ {
		if int(keys[i]) < len(stringTable) && int(vals[i]) < len(stringTable) {
			key := string(stringTable[keys[i]])
			value := string(stringTable[vals[i]])

			m.analyzeField(layer, key, value)
		}
	}
}

func (m *MBT) analyzeDenseNodes(dense *proto.DenseNodes, stringTable [][]byte) {
	layer := "dense_nodes"
	if _, exists := m.layerStats[layer]; !exists {
		m.layerStats[layer] = &LayerStat{
			Layer:    layer,
			Geometry: "Point",
			Count:    0,
		}
		m.fieldTypes[layer] = make(map[string]string)
	}

	kvIndex := 0
	keysVals := dense.GetKeysVals()

	for i := 0; i < len(dense.GetId()); i++ {
		m.layerStats[layer].Count++

		for kvIndex < len(keysVals) && keysVals[kvIndex] != 0 {
			keyIdx := keysVals[kvIndex]
			kvIndex++
			if kvIndex < len(keysVals) {
				valIdx := keysVals[kvIndex]
				kvIndex++

				if int(keyIdx) < len(stringTable) && int(valIdx) < len(stringTable) {
					key := string(stringTable[keyIdx])
					value := string(stringTable[valIdx])
					m.analyzeField(layer, key, value)
				}
			}
		}
		kvIndex++
	}
}

func (m *MBT) analyzeField(layer, key, value string) {
	fieldType := determineFieldType(value)

	if currentType, exists := m.fieldTypes[layer][key]; !exists {
		m.fieldTypes[layer][key] = fieldType
	} else if currentType != fieldType {
		m.fieldTypes[layer][key] = "string"
	}
}

func determineFieldType(value string) string {
	if _, err := fmt.Sscanf(value, "%f"); err == nil {
		return "number"
	}

	if value == "true" || value == "false" || value == "yes" || value == "no" {
		return "boolean"
	}

	return "string"
}

func (m *MBT) FinalizeMetadata() error {
	var vectorLayers []VectorLayer
	var layerStats []LayerStat

	for layerName, stats := range m.layerStats {
		vectorLayer := VectorLayer{
			ID:      layerName,
			MinZoom: 0,
			MaxZoom: 14,
			Fields:  make(map[string]string),
		}

		for fieldName, fieldType := range m.fieldTypes[layerName] {
			vectorLayer.Fields[fieldName] = fieldType
		}

		stats.AttributeCount = len(m.fieldTypes[layerName])
		stats.Attributes = m.collectAttributeStats(layerName)

		vectorLayers = append(vectorLayers, vectorLayer)
		layerStats = append(layerStats, *stats)
	}

	jsonData := map[string]interface{}{
		"vector_layers": vectorLayers,
		"tilestats": TileStats{
			LayerCount: len(vectorLayers),
			Layers:     layerStats,
		},
	}

	jsonBytes, err := json.MarshalIndent(jsonData, "", "    ")
	if err != nil {
		return err
	}

	stmt, err := m.db.Prepare("INSERT OR REPLACE INTO metadata (name, value) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer func(stmt *sql.Stmt) {
		err = stmt.Close()
		panic(err)
	}(stmt)

	_, err = stmt.Exec("json", string(jsonBytes))
	return err
}

func (m *MBT) collectAttributeStats(layer string) []Attribute {
	var attributes []Attribute

	for fieldName, fieldType := range m.fieldTypes[layer] {
		attr := Attribute{
			Attribute: fieldName,
			Type:      fieldType,
			Count:     1,
		}
		attributes = append(attributes, attr)
	}

	return attributes
}

func (m *MBT) Close() error {
	return m.db.Close()
}
