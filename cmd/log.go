package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrank/zdocker/container"
)

func NewLogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Fetch the logs of a container",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 0 {
				return errors.New("logs requires 1 argument")
			}
			containerName := args[0]
			ListContainerLogs(containerName)
			return nil
		},
	}

	return cmd
}

func ListContainerLogs(containerName string) {
	dirUrl := fmt.Sprintf(container.DefaultLocation, containerName)
	logFile := dirUrl + container.ContainerLogFile
	file, err := os.Open(logFile)
	defer file.Close()

	if err != nil {
		log.Errorf("Log container open file %s error %v", logFile, err)
		return
	}
	content, err := io.ReadAll(file)
	if err != nil {
		log.Errorf("Log container read file %s error %v", logFile, err)
		return
	}
	fmt.Fprint(os.Stdout, string(content))
}
