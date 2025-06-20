package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bytedance/sonic"
	_ "github.com/crazyfrank/zdocker/nsenter"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrank/zdocker/container"
)

const EnvExecPID = "zdocker_pid"
const EnvExecCMD = "zdocker_cmd"

func NewExecCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "exec a command into container",
		RunE: func(cmd *cobra.Command, args []string) error {
			// This is for callback
			if os.Getenv(EnvExecPID) != "" {
				log.Infof("pid callback pid %d", os.Getgid())
				return nil
			}
			// we expected zdocker exec [container] [command]
			if len(args) < 2 {
				return errors.New("missing container name or command")
			}
			containerName := args[0]
			commands := args[1:]

			ExecContainer(containerName, commands)

			return nil
		},
	}

	return cmd
}

func ExecContainer(containerName string, commands []string) {
	pid, err := getContainerPIDByName(containerName)
	if err != nil {
		log.Errorf("Exec container getContainerPIDByName %s error %v", containerName, err)
		return
	}

	cmds := strings.Join(commands, " ")
	log.Infof("container pid %s", pid)
	log.Infof("command %s", cmds)

	cmd := exec.Command("/proc/self/exe", "exec")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	os.Setenv(EnvExecPID, pid)
	os.Setenv(EnvExecCMD, cmds)

	if err := cmd.Run(); err != nil {
		log.Errorf("Exec container %s error %v", containerName, err)
	}
}

func getContainerPIDByName(containerName string) (string, error) {
	dirUrl := fmt.Sprintf(container.DefaultLocation, containerName)
	cfgFile := dirUrl + container.ConfigName
	content, err := os.ReadFile(cfgFile)
	if err != nil {
		return "", err
	}

	var containerInfo container.ContainerInfo
	if err := sonic.Unmarshal(content, &containerInfo); err != nil {
		return "", err
	}

	return containerInfo.PID, nil
}
