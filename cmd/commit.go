package cmd

import (
	"errors"
	"fmt"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	imageName string
)

func init() {
	commitCmd.Flags().StringVarP(&imageName, "image", "i", "", "commit image name")

	rootCmd.AddCommand(commitCmd)
}

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "commit a container into image",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("missing container name and image name")
		}
		commitContainer(args[0])
		return nil
	},
}

func commitContainer(imageName string) {
	mntUrl := "/root/mnt"
	imageTar := fmt.Sprintf("/root/%s.tar", imageName)
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntUrl, ".").CombinedOutput(); err != nil {
		log.Errorf("tar folder %s error. %v", mntUrl, err)
	}
}
