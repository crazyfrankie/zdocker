package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrankie/zdocker/container"
	_ "github.com/crazyfrankie/zdocker/nsenter"
)

const EnvExecPID = "zdocker_pid"
const EnvExecCMD = "zdocker_cmd"

func NewExecCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [CONTAINER] [COMMAND] [ARG...]",
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
		DisableFlagsInUseLine: true,
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
	containerEnvs := getEnvsByPid(pid)
	cmd.Env = append(cmd.Environ(), containerEnvs...)

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

func getEnvsByPid(pid string) []string {
	path := fmt.Sprintf("/proc/%s/environ", pid)
	content, err := os.ReadFile(path)
	if err != nil {
		log.Errorf("Read file %s error %v", path, err)
		return nil
	}
	//env split by \u0000
	envs := strings.Split(string(content), "\u0000")
	return envs
}
