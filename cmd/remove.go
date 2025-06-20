package cmd

import (
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrank/zdocker/container"
)

func NewRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm",
		Short: "remove a container",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("missing container name")
			}
			removeContainer(args[0])
			return nil
		},
	}

	return cmd
}

func removeContainer(containerName string) {
	info, err := getContainerInfoByName(containerName)
	if err != nil {
		log.Errorf("get container info by name %s error %v.", containerName, err)
		return
	}
	if info.Status != container.STOP {
		log.Errorf("cannot remove running container")
		return
	}

	dirUrl := fmt.Sprintf(container.DefaultLocation, containerName)
	if err := os.RemoveAll(dirUrl); err != nil {
		log.Errorf("Remove file %s error %v", dirUrl, err)
		return
	}
	container.DeleteWorkSpace(containerName, info.Volume)
}
