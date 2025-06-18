package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// RunContainerInitProcess 容器内部执行初始化程序
func RunContainerInitProcess() error {
	commands := readUserCommand()
	if commands == nil || len(commands) == 0 {
		return fmt.Errorf("run container get user command error, commands is nil")
	}

	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	path, err := exec.LookPath(commands[0])
	if err != nil {
		log.Errorf("Exec loop path error %v", err)
		return err
	}
	log.Infof("Find path %s", path)
	if err := syscall.Exec(path, commands[0:], os.Environ()); err != nil {
		log.Errorf(err.Error())
	}
	return nil
}

func readUserCommand() []string {
	pipe := os.NewFile(uintptr(3), "pipe")
	msg, err := io.ReadAll(pipe)
	if err != nil {
		log.Errorf("init read pipe error %v", err)
		return nil
	}
	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}
