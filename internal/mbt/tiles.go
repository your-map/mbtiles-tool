package mbt

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/your-map/mbtiles-tool/internal/osm/proto"
)

func (m *MBT) createSimpleTiles(points []struct {
	lat, lon float64
	tags     map[string]string
}, stmt *sql.Stmt) error {

	// Группируем точки по тайлам для разных zoom уровней
	for zoom := 0; zoom <= 14; zoom++ {
		tileMap := make(map[string][][]byte) // tileKey -> список точек

		for _, point := range points {
			tileX, tileY := m.deg2num(point.lat, point.lon, zoom)
			tileKey := fmt.Sprintf("%d/%d/%d", zoom, tileX, tileY)

			// Создаем простой GeoJSON для точки
			geojson := fmt.Sprintf(`{"type":"Point","coordinates":[%f,%f]}`, point.lon, point.lat)
			tileMap[tileKey] = append(tileMap[tileKey], []byte(geojson))
		}

		// Сохраняем тайлы
		for tileKey, features := range tileMap {
			var zoom, x, y int
			fmt.Sscanf(tileKey, "%d/%d/%d", &zoom, &x, &y)

			// Создаем простой FeatureCollection
			var featureStrs []string
			for _, feature := range features {
				featureStrs = append(featureStrs, fmt.Sprintf(`{"type":"Feature","geometry":%s}`, string(feature)))
			}

			featureCollection := fmt.Sprintf(`{"type":"FeatureCollection","features":[%s]}`, strings.Join(featureStrs, ","))

			_, err := stmt.Exec(zoom, x, y, []byte(featureCollection))
			if err != nil {
				return fmt.Errorf("failed to insert tile %s: %w", tileKey, err)
			}
		}
	}

	return nil
}

// Вспомогательные методы
func (m *MBT) decodeCoordinates(lat, lon int64, block *proto.PrimitiveBlock) (float64, float64) {
	granularity := float64(block.GetGranularity())
	latOffset := float64(block.GetLatOffset())
	lonOffset := float64(block.GetLonOffset())

	latitude := 1e-9 * (latOffset + (granularity * float64(lat)))
	longitude := 1e-9 * (lonOffset + (granularity * float64(lon)))

	return latitude, longitude
}

func (m *MBT) deg2num(lat, lon float64, zoom int) (int, int) {
	latRad := lat * math.Pi / 180.0
	n := math.Pow(2.0, float64(zoom))
	x := int((lon + 180.0) / 360.0 * n)
	y := int((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n)
	return x, y
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

func (m *MBT) extractDenseNodes(dense *proto.DenseNodes, block *proto.PrimitiveBlock, stringTable [][]byte) []struct {
	lat, lon float64
	tags     map[string]string
} {
	var points []struct {
		lat, lon float64
		tags     map[string]string
	}

	ids := dense.GetId()
	lats := dense.GetLat()
	lons := dense.GetLon()
	keysVals := dense.GetKeysVals()

	var currentId int64
	var currentLat int64
	var currentLon int64
	kvIndex := 0

	for i := 0; i < len(ids); i++ {
		currentId += ids[i]
		currentLat += lats[i]
		currentLon += lons[i]

		lat, lon := m.decodeCoordinates(currentLat, currentLon, block)
		tags := make(map[string]string)

		// Обработка тегов для dense nodes
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

		points = append(points, struct {
			lat, lon float64
			tags     map[string]string
		}{lat, lon, tags})
	}

	return points
}
