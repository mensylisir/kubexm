package images

import (
	"github.com/spf13/cobra"
)

// ImagesCmd represents the images command group
var ImagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Manage container images",
	Long:  `Commands for pushing and managing container images in private registries.`,
}

// AddImagesCommand adds the images command to the parent command.
func AddImagesCommand(parentCmd *cobra.Command) {
	parentCmd.AddCommand(ImagesCmd)
}
