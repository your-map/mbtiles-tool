package tiles

type Format string

var (
	MBT     Format = "mbt"
	OSM     Format = "osm"
	Unknown Format = "unknown"
)

var FormatFileExt = map[Format]string{
	MBT: ".mbtiles",
	OSM: ".osm.pbf",
}
