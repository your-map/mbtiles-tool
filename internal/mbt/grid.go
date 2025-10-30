package mbt

import (
	"fmt"

	"github.com/your-map/mbtiles-tool/internal/osm/proto"
)

type GridBox struct {
	HeaderBox *proto.HeaderBBox
}

func NewGrid(bbox *proto.HeaderBBox) *GridBox {
	return &GridBox{
		HeaderBox: bbox,
	}
}

func (gb *GridBox) Execute() (string, string) {
	left := float64(gb.HeaderBox.GetLeft()) / 1e9
	right := float64(gb.HeaderBox.GetRight()) / 1e9
	top := float64(gb.HeaderBox.GetTop()) / 1e9
	bottom := float64(gb.HeaderBox.GetBottom()) / 1e9

	centerLon := (left + right) / 2
	centerLat := (bottom + top) / 2

	bounds := fmt.Sprintf("%f,%f,%f,%f", left, bottom, right, top)
	center := fmt.Sprintf("%f,%f,%d", centerLon, centerLat, 10)

	return bounds, center
}
