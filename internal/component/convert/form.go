package convert

import (
	"errors"
	"os"

	"github.com/charmbracelet/huh"
)

var (
	errCpFile     = errors.New(`cannot copy file`)
	errGetHomeDir = errors.New(`cannot get home directory`)
)

func Run() (*Fields, error) {
	fields := &Fields{}

	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, errGetHomeDir
	}

	allowedTypes := []string{".osm.pbf"}

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewFilePicker().
				Title("osm.pbf file").
				Description("Select pbf file").
				CurrentDirectory(homedir).
				AllowedTypes(allowedTypes).
				Value(&fields.File).
				Picking(true),
		),
	).WithShowHelp(true).Run()
	if err != nil {
		return nil, errCpFile
	}

	return fields, nil
}
