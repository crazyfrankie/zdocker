package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrank/zdocker/container"
)

func NewStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stop a running container",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("missing container name")
			}
			stopContainer(args[0])
			return nil
		},
	}

	return cmd
}

func stopContainer(containerName string) {
	// get container pid for kill
	pid, err := getContainerPIDByName(containerName)
	if err != nil {
		log.Errorf("get container pid by name %s error %v.", containerName, err)
		return
	}
	pidInt, err := strconv.Atoi(pid)
	if err != nil {
		log.Errorf("convert pid from string to int error %v.", err)
		return
	}

	// kill container process
	if err := syscall.Kill(pidInt, syscall.SIGTERM); err != nil {
		log.Errorf("stop container %s error %v.", containerName, err)
		return
	}
	// modify container info
	info, err := getContainerInfoByName(containerName)
	if err != nil {
		log.Errorf("get container info by name %s error %v.", containerName, err)
		return
	}
	info.Status = container.STOP
	info.PID = ""

	newContent, err := sonic.Marshal(info)
	if err != nil {
		log.Errorf("json marshal %s error %v.", containerName, err)
		return
	}
	dirUrl := fmt.Sprintf(container.DefaultLocation, containerName)
	cfgFile := dirUrl + container.ConfigName
	if err := os.WriteFile(cfgFile, newContent, 0622); err != nil {
		log.Errorf("write file %s error %v.", cfgFile, err)
	}
}

func getContainerInfoByName(containerName string) (*container.ContainerInfo, error) {
	dirUrl := fmt.Sprintf(container.DefaultLocation, containerName)
	cfgFile := dirUrl + container.ConfigName
	content, err := os.ReadFile(cfgFile)
	if err != nil {
		log.Errorf("read file %s error %v.", cfgFile, err)
		return nil, err
	}

	var info container.ContainerInfo
	if err := sonic.Unmarshal(content, &info); err != nil {
		log.Errorf("getContainerInfoByName unmarshal error %v.", err)
		return nil, err
	}

	return &info, nil
}
