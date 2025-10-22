package command

import (
	"github.com/spf13/cobra"
	"github.com/your-map/mbtiles-tool/configs/constname"
	"github.com/your-map/mbtiles-tool/internal/component/output"
	"github.com/your-map/mbtiles-tool/pkg/tiles"
)

// convertCmd Command for build pipeline
var convertCmd = &cobra.Command{
	Use:   constname.UseConvertCmd,
	Short: constname.ShortConvertCmd,
	Long:  constname.LongConvertCmd,
	RunE: func(cmd *cobra.Command, args []string) error {
		//todo delete after completed
		//fields, err := convertForm.Run()
		//if err != nil {
		//	return err
		//}

		pbfMap := tiles.NewMap("test/maps/andorra.osm.pbf")

		mbtMap, err := pbfMap.Convert()
		if err != nil {
			return err
		}

		output.Green("Success convert file: ", mbtMap.File)

		return nil
	},
}
