package container

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	log "github.com/sirupsen/logrus"
)

var (
	RUNNING = "running"
	STOP    = "stop"
	EXIT    = "exit"

	DefaultLocation  = "/var/run/zdocker/%s/"
	ConfigName       = "config.json"
	ContainerLogFile = "container.log"
)

type ContainerInfo struct {
	PID        string `json:"pid"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Command    string `json:"command"`
	CreateTime string `json:"createTime"`
	Status     string `json:"status"`
}

// NewParentProcess Build a new cmd that creates the container process.
func NewParentProcess(tty bool, containerName string, volume string) (*exec.Cmd, *os.File) {
	readPipe, writePipe, err := newPipe()
	if err != nil {
		log.Errorf("New pipe error %v", err)
		return nil, nil
	}
	cmd := exec.Command("/proc/self/exe", "init")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		dirUrl := fmt.Sprintf(DefaultLocation, containerName)
		if err := os.MkdirAll(dirUrl, 0622); err != nil {
			log.Errorf("NewParentProcess mkdir %s error %v.", dirUrl, err)
			return nil, nil
		}
		logFile := dirUrl + ContainerLogFile
		stdLogFile, err := os.Create(logFile)
		if err != nil {
			log.Errorf("NewParentProcess create file %s error %v", logFile, err)
			return nil, nil
		}
		cmd.Stdout = stdLogFile
	}
	cmd.ExtraFiles = []*os.File{readPipe}
	rootUrl, mntUrl := "/root", "/root/mnt"
	NewWorkSpace(rootUrl, mntUrl, volume)
	cmd.Dir = mntUrl
	return cmd, writePipe
}

func newPipe() (*os.File, *os.File, error) {
	read, write, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	return read, write, nil
}
