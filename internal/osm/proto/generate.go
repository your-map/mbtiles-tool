package proto

//go:generate protoc  --proto_path=. --go_opt=module=github.com/your-map/mbtiles-tool/internal/osm/proto  --go_out=.  fileformat.proto osmformat.proto
