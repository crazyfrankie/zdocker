package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrank/zdocker/container"
)

var enableTTY bool

func init() {
	runCmd.Flags().BoolVarP(&enableTTY, "ti", "t", false, "enable tty")

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
		Run(enableTTY, args[0])

		return nil
	},
}

func Run(tty bool, command string) {
	// 构建创建容器的父进程
	parent := container.NewParentProcess(tty, command)
	if err := parent.Start(); err != nil { // 执行创建容器
		log.Error(err)
	}
	parent.Wait()
	os.Exit(-1)
}
