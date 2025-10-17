package command

import (
	"github.com/spf13/cobra"
	"github.com/your-map/mbtiles-tool/configs/constname"
	"github.com/your-map/mbtiles-tool/internal/component/form"
	"github.com/your-map/mbtiles-tool/internal/component/output"
)

// convertCmd Command for build pipeline
var convertCmd = &cobra.Command{
	Use:   constname.UseConvertCmd,
	Short: constname.ShortConvertCmd,
	Long:  constname.LongConvertCmd,
	RunE: func(cmd *cobra.Command, args []string) error {
		fields, err := form.Run()
		if err != nil {
			return err
		}

		output.Green("Success green output: ", fields.ExampleInput)
		output.Red("This command will be run successfully")

		return nil
	},
}
