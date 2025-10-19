package osmpbf

//go:generate protoc  --proto_path=. --go_opt=module=github.com/your-map/mbtiles-tool/internal/osmpbf  --go_out=.  fileformat.proto osmformat.proto
