package mbt

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

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
		if err != nil {
			log.Fatal(err)
		}
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
