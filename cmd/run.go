package cmd

import (
	"errors"
	"fmt"
	"github.com/crazyfrank/zdocker/network"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrank/zdocker/cgroups"
	"github.com/crazyfrank/zdocker/container"
)

const (
	letterBytes = "1234567890"
)

type runOptions struct {
	detach        bool
	enableTTY     bool
	containerName string
	volume        string
	memoryLimit   string
	cpuShareLimit string
	cpuSetLimit   string
	network       string
	environments  []string
	portMapping   []string
}

func NewRunCommand() *cobra.Command {
	var option runOptions

	cmd := &cobra.Command{
		Use:          "run [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short:        "Create a container with namespace and cgroups limit ie: zdocker run -t [command]",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("missing container command")
			}
			if option.enableTTY && option.detach {
				return errors.New("t and d parameter can not both provided")
			}
			Run(option, args, &cgroups.ResourceConfig{
				MemoryLimit: option.memoryLimit,
				CpuShare:    option.cpuShareLimit,
				CpuSet:      option.cpuSetLimit,
			})

			return nil
		},
		DisableFlagsInUseLine: true,
	}

	flags := cmd.Flags()
	flags.SetInterspersed(false)
	flags.BoolVarP(&option.detach, "detach", "d", false, "detach container")
	flags.BoolVarP(&option.enableTTY, "ti", "t", false, "enable tty")
	flags.StringVarP(&option.containerName, "name", "n", "", "container name")
	flags.StringVarP(&option.volume, "volume", "v", "", "volume")
	flags.StringVarP(&option.memoryLimit, "memory", "m", "", "memory limit")
	flags.StringVarP(&option.cpuShareLimit, "cpushare", "", "", "cpushare limit")
	flags.StringVarP(&option.cpuSetLimit, "cpuset", "", "", "cpuset limit")
	flags.StringVarP(&option.network, "net", "", "", "container network")
	flags.StringArrayVarP(&option.portMapping, "port", "p", []string{}, "port mapping")
	flags.StringArrayVarP(&option.environments, "env", "e", []string{}, "container running env (e.g., -e KEY1=value1 -e KEY2=value2)")

	return cmd
}

func Run(options runOptions, args []string, res *cgroups.ResourceConfig) {
	// get image name
	imageName := args[0]
	commands := args[1:]

	// Handle default commands like Docker does
	if len(commands) == 0 {
		commands = getDefaultCommand(imageName, options.detach)
	}

	containerID := randStringBytes(10)
	if options.containerName == "" {
		options.containerName = containerID
	}

	// build the parent process that created the container
	parent, writePipe := container.NewParentProcess(imageName, options.containerName, options.volume, options.enableTTY, options.environments)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}
	if err := parent.Start(); err != nil {
		log.Error(err)
	}

	// record container info
	if err := recordContainerInfo(parent.Process.Pid, containerID, options.containerName, options.volume, commands); err != nil {
		log.Errorf("record container info error %v.", err)
		return
	}

	cgroupManager := cgroups.NewCgroupManager("zdocker")
	defer cgroupManager.Destroy()

	cgroupManager.Set(res)
	cgroupManager.Apply(parent.Process.Pid)

	if options.network != "" {
		// config container network
		network.InitNetwork()
		containerInfo := &container.ContainerInfo{
			ID:          containerID,
			PID:         strconv.Itoa(parent.Process.Pid),
			Name:        options.containerName,
			PortMapping: options.portMapping,
		}
		if err := network.Connect(options.network, containerInfo); err != nil {
			log.Errorf("Error Connect Network %v", err)
		}
	}

	sendInitCommand(commands, writePipe)
	if options.enableTTY {
		parent.Wait()
		deleteContainerInfo(options.containerName)
		container.DeleteWorkSpace(options.containerName, options.volume)
	} else {
		// For detach mode, we don't wait for the container to finish
		log.Infof("Container %s is running in detach mode with PID %d", options.containerName, parent.Process.Pid)
	}
}

// getDefaultCommand returns default commands for different images like Docker does
func getDefaultCommand(imageName string, isDetach bool) []string {
	switch imageName {
	case "busybox":
		if isDetach {
			// For detach mode, use a command that keeps the container running
			return []string{"sleep", "infinity"}
		} else {
			// For interactive mode, use shell
			return []string{"sh"}
		}
	default:
		// For unknown images, try to keep them running in detach mode
		if isDetach {
			return []string{"sleep", "infinity"}
		} else {
			return []string{"sh"}
		}
	}
}

func sendInitCommand(commands []string, writePipe *os.File) {
	command := strings.Join(commands, " ")
	log.Infof("command all is %s", command)
	writePipe.WriteString(command)
	writePipe.Close()
}

func recordContainerInfo(pid int, containerId string, containerName string, volume string, commands []string) error {
	createTime := time.Now().Format(time.DateTime)
	command := strings.Join(commands, " ")
	// if user not pick container name, then use cid as container name
	containerInfo := &container.ContainerInfo{
		PID:        strconv.Itoa(pid),
		ID:         containerId,
		Name:       containerName,
		Command:    command,
		CreateTime: createTime,
		Status:     container.RUNNING,
		Volume:     volume,
	}
	data, err := sonic.Marshal(containerInfo)
	if err != nil {
		log.Errorf("record container info error %v", err)
		return err
	}
	json := string(data)
	// container info path
	dirUrl := fmt.Sprintf(container.DefaultLocation, containerName)
	if err := os.MkdirAll(dirUrl, 0622); err != nil {
		log.Errorf("mkdir error %s error %v.", dirUrl, err)
		return err
	}
	fileName := dirUrl + "/" + container.ConfigName
	// create config.json
	file, err := os.Create(fileName)
	if err != nil {
		log.Errorf("create file %s error %v.", fileName, err)
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(json); err != nil {
		log.Errorf("file write string error %v", err)
		return err
	}

	return nil
}

func deleteContainerInfo(containerName string) {
	dirUrl := container.DefaultLocation + "/" + containerName
	if err := os.RemoveAll(dirUrl); err != nil {
		log.Errorf("remove dir %s error %v.", dirUrl, err)
	}
}

func randStringBytes(n int) string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	var res strings.Builder
	for i := 0; i < n; i++ {
		res.WriteString(strconv.Itoa(rand.Intn(len(letterBytes))))
	}

	return res.String()
}
