package cmd

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrank/zdocker/cgroups"
	"github.com/crazyfrank/zdocker/container"
)

var (
	enableTTY     bool
	memoryLimit   string
	cpuShareLimit string
	cpuSetLimit   string
)

func init() {
	runCmd.Flags().SetInterspersed(false)
	runCmd.Flags().BoolVarP(&enableTTY, "ti", "t", false, "enable tty")
	runCmd.Flags().StringVarP(&memoryLimit, "memory", "m", "", "memory limit")
	runCmd.Flags().StringVarP(&cpuShareLimit, "cpushare", "", "", "cpushare limit")
	runCmd.Flags().StringVarP(&cpuSetLimit, "cpuset", "", "", "cpuset limit")

	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Create a container with namespace and cgroups limit ie: zdocker run -ti [command]",
	// 命令出错时，不打印帮助信息。设置为 true 可以确保命令出错时一眼就能看到错误信息
	SilenceUsage: true,
	// 指定调用 cmd.Execute() 时，执行的 Run 函数
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("missing container command")
		}
		Run(enableTTY, args, &cgroups.ResourceConfig{
			MemoryLimit: memoryLimit,
			CpuShare:    cpuShareLimit,
			CpuSet:      cpuSetLimit,
		})

		return nil
	},
}

func Run(tty bool, commands []string, res *cgroups.ResourceConfig) {
	// 构建创建容器的父进程
	parent, writePipe := container.NewParentProcess(tty)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}
	if err := parent.Start(); err != nil { // 执行创建容器
		log.Error(err)
	}
	cgroupManager := cgroups.NewCgroupManager("zdocker")
	defer cgroupManager.Destroy()

	cgroupManager.Set(res)
	cgroupManager.Apply(parent.Process.Pid)

	sendInitCommand(commands, writePipe)
	parent.Wait()
	root, mnt := "/root", "/root/mnt"
	container.DeleteWorkSpace(root, mnt)
	os.Exit(0)
}

func sendInitCommand(commands []string, writePipe *os.File) {
	command := strings.Join(commands, " ")
	log.Infof("command all is %s", command)
	writePipe.WriteString(command)
	writePipe.Close()
}
