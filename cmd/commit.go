package cmd

import (
	"errors"
	"fmt"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrankie/zdocker/container"
)

type commitOptions struct {
	imageName string
}

func NewCommitCommand() *cobra.Command {
	var option commitOptions

	cmd := &cobra.Command{
		Use:   "commit [CONTAINER] [IMAGE]",
		Short: "Commit a container into image",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("missing container name and image name")
			}
			commitContainer(args[0], args[1])
			return nil
		},
		DisableFlagsInUseLine: true,
	}
	flags := cmd.Flags()
	flags.StringVarP(&option.imageName, "image", "i", "", "commit image name")

	return cmd
}

func commitContainer(containerName string, imageName string) {
	mntUrl := fmt.Sprintf(container.MntUrl, containerName)
	mntUrl += "/"

	imageTar := fmt.Sprintf("%s/%s.tar", container.RootUrl, imageName)

	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntUrl, ".").CombinedOutput(); err != nil {
		log.Errorf("tar folder %s error. %v", mntUrl, err)
	}
}
