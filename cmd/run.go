package cmd

import (
	"errors"
	"fmt"
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

var (
	detach        bool
	enableTTY     bool
	containerName string
	volume        string
	memoryLimit   string
	cpuShareLimit string
	cpuSetLimit   string
)

func init() {
	runCmd.Flags().SetInterspersed(false)
	runCmd.Flags().BoolVarP(&detach, "detach", "d", false, "detach container")
	runCmd.Flags().BoolVarP(&enableTTY, "ti", "t", false, "enable tty")
	runCmd.Flags().StringVarP(&containerName, "name", "n", "", "container name")
	runCmd.Flags().StringVarP(&volume, "volume", "v", "", "volume")
	runCmd.Flags().StringVarP(&memoryLimit, "memory", "m", "", "memory limit")
	runCmd.Flags().StringVarP(&cpuShareLimit, "cpushare", "", "", "cpushare limit")
	runCmd.Flags().StringVarP(&cpuSetLimit, "cpuset", "", "", "cpuset limit")

	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:          "run",
	Short:        "Create a container with namespace and cgroups limit ie: zdocker run -ti [command]",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing container command")
		}
		if enableTTY && detach {
			return errors.New("t and d parameter can not both provided")
		}
		Run(containerName, volume, enableTTY, args, &cgroups.ResourceConfig{
			MemoryLimit: memoryLimit,
			CpuShare:    cpuShareLimit,
			CpuSet:      cpuSetLimit,
		})

		return nil
	},
}

func Run(containerName string, volume string, tty bool, commands []string, res *cgroups.ResourceConfig) {
	// build the parent process that created the container
	parent, writePipe := container.NewParentProcess(tty, volume)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}
	if err := parent.Start(); err != nil {
		log.Error(err)
	}
	var err error
	containerName, err = recordContainerInfo(parent.Process.Pid, containerName, commands)
	if err != nil {
		log.Errorf("record container info error %v.", err)
		return
	}

	cgroupManager := cgroups.NewCgroupManager("zdocker")
	defer cgroupManager.Destroy()

	cgroupManager.Set(res)
	cgroupManager.Apply(parent.Process.Pid)

	sendInitCommand(commands, writePipe)
	if tty {
		parent.Wait()
		deleteContainerInfo(containerName)
	}
	rootUrl, mntUrl := "/root", "/root/mnt"
	container.DeleteWorkSpace(rootUrl, mntUrl, volume)
	os.Exit(0)
}

func sendInitCommand(commands []string, writePipe *os.File) {
	command := strings.Join(commands, " ")
	log.Infof("command all is %s", command)
	writePipe.WriteString(command)
	writePipe.Close()
}

func recordContainerInfo(pid int, containerName string, commands []string) (string, error) {
	cid := randStringBytes(10)
	createTime := time.Now().Format(time.DateTime)
	command := strings.Join(commands, "")
	// if user not pick container name, then use cid as container name
	if containerName == "" {
		containerName = cid
	}
	containerInfo := &container.ContainerInfo{
		PID:        strconv.Itoa(pid),
		ID:         cid,
		Name:       containerName,
		Command:    command,
		CreateTime: createTime,
		Status:     container.RUNNING,
	}
	data, err := sonic.Marshal(containerInfo)
	if err != nil {
		log.Errorf("record container info error %v", err)
		return "", err
	}
	json := string(data)
	// container info path
	dirUrl := fmt.Sprintf(container.DefaultLocation, containerName)
	if err := os.MkdirAll(dirUrl, 0622); err != nil {
		log.Errorf("mkdir error %s error %v.", dirUrl, err)
		return "", err
	}
	fileName := dirUrl + "/" + container.ConfigName
	// create config.json
	file, err := os.Create(fileName)
	if err != nil {
		log.Errorf("create file %s error %v.", fileName, err)
		return "", err
	}
	defer file.Close()

	if _, err := file.WriteString(json); err != nil {
		log.Errorf("file write string error %v", err)
		return "", err
	}

	return containerName, nil
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
