package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrankie/zdocker/container"
)

type stopOptions struct {
	signal  string
	timeout int
}

func NewStopCommand() *cobra.Command {
	var option stopOptions

	cmd := &cobra.Command{
		Use:   "stop [OPTIONS] [CONTAINER]",
		Short: "stop a running container",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("missing container name")
			}
			return stopContainer(args[0], option.timeout, option.signal)
		},
		DisableFlagsInUseLine: true,
	}

	flags := cmd.Flags()
	flags.IntVarP(&option.timeout, "timeout", "t", 0, "seconds to wait before killing the container")
	flags.StringVarP(&option.signal, "signal", "s", "SIGTERM", "signal to send to the container")

	return cmd
}

func stopContainer(containerName string, timeout int, signal string) error {
	// get container info first to check status
	info, err := getContainerInfoByName(containerName)
	if err != nil {
		return fmt.Errorf("get container info by name %s error %v", containerName, err)
	}

	// check if container is already stopped
	if info.Status == container.STOP {
		log.Infof("Container %s is already stopped", containerName)
		return nil
	}

	if info.PID == "" {
		return fmt.Errorf("container %s has no PID information", containerName)
	}

	pidInt, err := strconv.Atoi(info.PID)
	if err != nil {
		return fmt.Errorf("convert pid from string to int error %v", err)
	}

	// parse signal
	sig, err := parseSignal(signal)
	if err != nil {
		return err
	}

	// kill container process
	if err := syscall.Kill(pidInt, sig); err != nil {
		return fmt.Errorf("send signal %s failed: %v", signal, err)
	}
	if sig == syscall.SIGKILL {
		log.Infof("Sent SIGKILL to container %s (immediate termination)", containerName)
		return updateContainerStatus(containerName)
	}

	log.Infof("Sent %s to container %s, waiting up to %d seconds...", signal, containerName, timeout)
	return waitForContainerStop(pidInt, containerName, timeout)
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

func updateContainerStatus(containerName string) error {
	// modify container info
	info, err := getContainerInfoByName(containerName)
	if err != nil {
		return fmt.Errorf("get container info by name %s error %v", containerName, err)
	}
	info.Status = container.STOP
	info.PID = ""

	newContent, err := sonic.Marshal(info)
	if err != nil {
		return fmt.Errorf("json marshal %s error %v", containerName, err)
	}
	dirUrl := fmt.Sprintf(container.DefaultLocation, containerName)
	cfgFile := dirUrl + container.ConfigName
	if err := os.WriteFile(cfgFile, newContent, 0622); err != nil {
		return fmt.Errorf("write file %s error %v", cfgFile, err)
	}

	return nil
}

func parseSignal(signalStr string) (syscall.Signal, error) {
	switch strings.ToUpper(signalStr) {
	case "SIGTERM", "TERM", "15":
		return syscall.SIGTERM, nil
	case "SIGKILL", "KILL", "9":
		return syscall.SIGKILL, nil
	case "SIGINT", "INT", "2":
		return syscall.SIGINT, nil
	case "SIGQUIT", "QUIT", "3":
		return syscall.SIGQUIT, nil
	case "SIGHUP", "HUP", "1":
		return syscall.SIGHUP, nil
	default:
		return 0, fmt.Errorf("unsupported signal: %s (supported: SIGTERM/15, SIGKILL/9, SIGINT/2, SIGQUIT/3, SIGHUP/1)", signalStr)
	}
}

func waitForContainerStop(pid int, containerName string, timeout int) error {
	stopped := make(chan bool)
	errCh := make(chan error)
	done := make(chan struct{}) // To notify the goroutine to stop

	go func() {
		defer func() {
			// Make sure the channel is closed no matter how you exit
			select {
			case <-done:
			default:
				close(done)
			}
		}()

		for {
			select {
			case <-done:
				return // Receive stop signal, exit goroutine.
			default:
				// check process existed
				if err := syscall.Kill(pid, 0); err != nil {
					if errors.Is(err, syscall.ESRCH) { // ESRCH indicates that the process does not exist
						stopped <- true
						return
					}
					errCh <- err
					return
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	if timeout == 0 {
		select {
		case <-stopped:
			close(done)
			log.Infof("Container %s stopped gracefully", containerName)
			return updateContainerStatus(containerName)
		case err := <-errCh:
			close(done)
			return fmt.Errorf("error while waiting for container stop: %v", err)
		}
	} else {
		select {
		case <-stopped:
			close(done)
			log.Infof("Container %s stopped gracefully", containerName)
			return updateContainerStatus(containerName)
		case err := <-errCh:
			close(done)
			return fmt.Errorf("error while waiting for container stop: %v", err)
		case <-time.After(time.Duration(timeout) * time.Second):
			close(done)
			log.Infof("Container %s not stopped after %d seconds, sending SIGKILL", containerName, timeout)
			if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
				return fmt.Errorf("send SIGKILL failed: %v", err)
			}
			return updateContainerStatus(containerName)
		}
	}
}
