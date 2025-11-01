package mbt

import (
	"database/sql"
	"fmt"
	"log"
	"math"

	_ "github.com/mattn/go-sqlite3"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/simplify"
	"github.com/your-map/mbtiles-tool/internal/osm/proto"
)

// Структуры для хранения данных OSM
type PointData struct {
	ID   int64
	Lat  float64
	Lon  float64
	Tags map[string]string
}

type WayData struct {
	ID    int64
	Refs  []int64
	Tags  map[string]string
	Nodes []*PointData
}

type MBT struct {
	db           *sql.DB
	layerStats   map[string]*LayerStat
	vectorLayers map[string]*VectorLayer
	fieldTypes   map[string]map[string]string

	// Кэши для данных OSM
	nodesCache map[int64]*PointData
	waysCache  []*WayData
	allPoints  []*PointData
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
		nodesCache:   make(map[int64]*PointData),
		waysCache:    make([]*WayData, 0),
		allPoints:    make([]*PointData, 0),
	}, nil
}

func (m *MBT) WriteMetaData(metaData *proto.HeaderBlock) error {
	stmt, err := m.db.Prepare("INSERT OR REPLACE INTO metadata (name, value) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

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

	// Обрабатываем данные и сохраняем в кэш
	for _, group := range data.GetPrimitivegroup() {
		// Обрабатываем обычные nodes
		for _, node := range group.GetNodes() {
			m.analyzePrimitive("nodes", node.GetKeys(), node.GetVals(), stringTable, "Point")

			point := m.processNode(node, data, stringTable)
			m.nodesCache[point.ID] = point
			m.allPoints = append(m.allPoints, point)
		}

		// Обрабатываем dense nodes
		if dense := group.GetDense(); dense != nil {
			m.analyzeDenseNodes(dense, stringTable)

			densePoints := m.processDenseNodes(dense, data, stringTable)
			for _, point := range densePoints {
				m.nodesCache[point.ID] = point
				m.allPoints = append(m.allPoints, point)
			}
		}

		// Обрабатываем ways
		for _, way := range group.GetWays() {
			m.analyzePrimitive("ways", way.GetKeys(), way.GetVals(), stringTable, "LineString")

			wayData := m.processWay(way, stringTable)
			m.waysCache = append(m.waysCache, wayData)
		}

		for _, relation := range group.GetRelations() {
			m.analyzePrimitive("relations", relation.GetKeys(), relation.GetVals(), stringTable, "GeometryCollection")
		}
	}

	return nil
}

// После обработки всех блоков вызываем генерацию тайлов
func (m *MBT) GenerateTiles() error {
	// Восстанавливаем геометрию для ways
	m.reconstructWayGeometry()

	// Генерируем тайлы для разных уровней масштабирования
	for zoom := 0; zoom <= 14; zoom++ {
		log.Printf("Generating tiles for zoom %d", zoom)
		err := m.generateTilesForZoom(zoom)
		if err != nil {
			return fmt.Errorf("failed to generate tiles for zoom %d: %w", zoom, err)
		}
	}

	return nil
}

func (m *MBT) generateTilesForZoom(zoom int) error {
	stmt, err := m.db.Prepare("INSERT OR REPLACE INTO tiles (zoom_level, tile_column, tile_row, tile_data) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	totalTiles := 1 << zoom // 2^zoom

	for x := 0; x < totalTiles; x++ {
		for y := 0; y < totalTiles; y++ {
			tileBounds := m.getTileBounds(x, y, zoom)

			// Находим объекты в bounding box тайла
			pointsInTile := m.findPointsInTile(tileBounds)
			waysInTile := m.findWaysInTile(tileBounds)

			// Создаем MVT тайл только если есть данные
			if len(pointsInTile) > 0 || len(waysInTile) > 0 {
				tileData, err := m.createMVTForTile(pointsInTile, waysInTile, zoom, x, y)
				if err != nil {
					return fmt.Errorf("failed to create MVT for tile %d/%d/%d: %w", zoom, x, y, err)
				}

				if len(tileData) > 0 {
					_, err = stmt.Exec(zoom, x, y, tileData)
					if err != nil {
						return fmt.Errorf("failed to save tile %d/%d/%d: %w", zoom, x, y, err)
					}
				}
			}
		}
	}

	return nil
}

// Основной метод создания MVT тайла
func (m *MBT) createMVTForTile(points []*PointData, ways []*WayData, zoom, x, y int) ([]byte, error) {
	// Создаем тайл
	tile := maptile.New(uint32(x), uint32(y), maptile.Zoom(zoom))

	// Создаем FeatureCollection для точек
	pointFeatures := make([]*geojson.Feature, 0)
	for _, point := range points {
		feature := geojson.NewFeature(orb.Point{point.Lon, point.Lat})
		feature.Properties = make(map[string]interface{})
		feature.Properties["id"] = point.ID
		feature.Properties["type"] = "node"
		for k, v := range point.Tags {
			feature.Properties[k] = v
		}
		pointFeatures = append(pointFeatures, feature)
	}

	// Создаем FeatureCollection для линий
	lineFeatures := make([]*geojson.Feature, 0)
	for _, way := range ways {
		if len(way.Nodes) >= 2 {
			lineString := make(orb.LineString, len(way.Nodes))
			for i, node := range way.Nodes {
				lineString[i] = orb.Point{node.Lon, node.Lat}
			}

			feature := geojson.NewFeature(lineString)
			feature.Properties = make(map[string]interface{})
			feature.Properties["id"] = way.ID
			feature.Properties["type"] = "way"
			for k, v := range way.Tags {
				feature.Properties[k] = v
			}
			lineFeatures = append(lineFeatures, feature)
		}
	}

	// Создаем слои MVT
	layers := make([]*mvt.Layer, 0)

	// Слой точек
	if len(pointFeatures) > 0 {
		pointCollection := &geojson.FeatureCollection{
			Features: pointFeatures,
		}
		pointLayer := &mvt.Layer{
			Name:     "points",
			Features: pointCollection.Features,
		}
		layers = append(layers, pointLayer)
	}

	// Слой линий
	if len(lineFeatures) > 0 {
		lineCollection := &geojson.FeatureCollection{
			Features: lineFeatures,
		}
		lineLayer := &mvt.Layer{
			Name:     "lines",
			Features: lineCollection.Features,
		}
		layers = append(layers, lineLayer)
	}

	if len(layers) == 0 {
		return []byte{}, nil
	}

	// Проецируем и упрощаем геометрию для тайла
	for _, layer := range layers {
		layer.ProjectToTile(tile)
		layer.Simplify(simplify.DouglasPeucker(1.0))
		layer.RemoveEmpty(1.0, 1.0)
	}

	// Кодируем в MVT
	dataMvt, err := mvt.Marshal(layers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MVT: %w", err)
	}

	return dataMvt, nil
}

// Вспомогательные методы
func (m *MBT) processNode(node *proto.Node, block *proto.PrimitiveBlock, stringTable [][]byte) *PointData {
	lat, lon := m.decodeCoordinates(node.GetLat(), node.GetLon(), block)

	return &PointData{
		ID:   node.GetId(),
		Lat:  lat,
		Lon:  lon,
		Tags: m.extractTags(node.GetKeys(), node.GetVals(), stringTable),
	}
}

func (m *MBT) processDenseNodes(dense *proto.DenseNodes, block *proto.PrimitiveBlock, stringTable [][]byte) []*PointData {
	var points []*PointData

	ids := dense.GetId()
	lats := dense.GetLat()
	lons := dense.GetLon()
	keysVals := dense.GetKeysVals()

	var currentID int64
	var currentLat int64
	var currentLon int64
	kvIndex := 0

	for i := 0; i < len(ids); i++ {
		currentID += ids[i]
		currentLat += lats[i]
		currentLon += lons[i]

		lat, lon := m.decodeCoordinates(currentLat, currentLon, block)
		tags := make(map[string]string)

		// Обрабатываем теги
		for kvIndex < len(keysVals) && keysVals[kvIndex] != 0 {
			keyIdx := keysVals[kvIndex]
			kvIndex++
			if kvIndex < len(keysVals) {
				valIdx := keysVals[kvIndex]
				kvIndex++

				if int(keyIdx) < len(stringTable) && int(valIdx) < len(stringTable) {
					key := string(stringTable[keyIdx])
					value := string(stringTable[valIdx])
					tags[key] = value
				}
			}
		}
		kvIndex++ // Пропускаем разделитель

		points = append(points, &PointData{
			ID:   currentID,
			Lat:  lat,
			Lon:  lon,
			Tags: tags,
		})
	}

	return points
}

func (m *MBT) processWay(way *proto.Way, stringTable [][]byte) *WayData {
	return &WayData{
		ID:   way.GetId(),
		Refs: way.GetRefs(),
		Tags: m.extractTags(way.GetKeys(), way.GetVals(), stringTable),
	}
}

func (m *MBT) reconstructWayGeometry() {
	for _, way := range m.waysCache {
		var nodes []*PointData
		for _, ref := range way.Refs {
			if node, exists := m.nodesCache[ref]; exists {
				nodes = append(nodes, node)
			}
		}
		way.Nodes = nodes
	}
}

func (m *MBT) decodeCoordinates(lat, lon int64, block *proto.PrimitiveBlock) (float64, float64) {
	granularity := float64(block.GetGranularity())
	latOffset := float64(block.GetLatOffset())
	lonOffset := float64(block.GetLonOffset())

	latitude := 1e-9 * (latOffset + (granularity * float64(lat)))
	longitude := 1e-9 * (lonOffset + (granularity * float64(lon)))

	return latitude, longitude
}

func (m *MBT) getTileBounds(x, y, zoom int) struct{ MinLat, MaxLat, MinLon, MaxLon float64 } {
	n := math.Pow(2.0, float64(zoom))

	minLon := float64(x)/n*360.0 - 180.0
	maxLon := float64(x+1)/n*360.0 - 180.0

	minLat := math.Atan(math.Sinh(math.Pi*(1-2*float64(y)/n))) * 180.0 / math.Pi
	maxLat := math.Atan(math.Sinh(math.Pi*(1-2*float64(y+1)/n))) * 180.0 / math.Pi

	return struct{ MinLat, MaxLat, MinLon, MaxLon float64 }{
		MinLat: minLat,
		MaxLat: maxLat,
		MinLon: minLon,
		MaxLon: maxLon,
	}
}

func (m *MBT) findPointsInTile(bounds struct{ MinLat, MaxLat, MinLon, MaxLon float64 }) []*PointData {
	var points []*PointData

	for _, point := range m.allPoints {
		if point.Lat >= bounds.MinLat && point.Lat <= bounds.MaxLat &&
			point.Lon >= bounds.MinLon && point.Lon <= bounds.MaxLon {
			points = append(points, point)
		}
	}

	return points
}

func (m *MBT) findWaysInTile(bounds struct{ MinLat, MaxLat, MinLon, MaxLon float64 }) []*WayData {
	var ways []*WayData

	for _, way := range m.waysCache {
		// Простая проверка - если хотя бы одна точка way попадает в тайл
		for _, node := range way.Nodes {
			if node.Lat >= bounds.MinLat && node.Lat <= bounds.MaxLat &&
				node.Lon >= bounds.MinLon && node.Lon <= bounds.MaxLon {
				ways = append(ways, way)
				break
			}
		}
	}

	return ways
}

func (m *MBT) extractTags(keys, vals []uint32, stringTable [][]byte) map[string]string {
	tags := make(map[string]string)
	for i := 0; i < len(keys) && i < len(vals); i++ {
		if int(keys[i]) < len(stringTable) && int(vals[i]) < len(stringTable) {
			key := string(stringTable[keys[i]])
			value := string(stringTable[vals[i]])
			tags[key] = value
		}
	}
	return tags
}

func (m *MBT) Close() error {
	return m.db.Close()
}
