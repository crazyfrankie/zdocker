package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"
	"text/tabwriter"

	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrankie/zdocker/container"
)

func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List all the containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			ListContainers()
			return nil
		},
		DisableFlagsInUseLine: true,
	}

	return cmd
}

func ListContainers() {
	dirUrl := fmt.Sprintf(container.DefaultLocation, "")
	dirUrl = dirUrl[:len(dirUrl)-1]
	files, err := os.ReadDir(dirUrl)
	if err != nil {
		log.Errorf("read dir %s error %v", dirUrl, err)
		return
	}
	containerInfos := make([]*container.ContainerInfo, 0, len(files))
	for _, f := range files {
		info, err := getContainerInfo(f)
		if err != nil {
			log.Errorf("get container info error %v.", err)
			continue
		}

		// Check if container process is still running and update status if needed
		if info.Status == container.RUNNING && info.PID != "" {
			if !isProcessRunning(info.PID) {
				if err := updateContainerStatusToExit(info.Name); err != nil {
					log.Errorf("Failed to update container status: %v", err)
				} else {
					info.Status = container.EXIT
					info.PID = ""
				}
			}
		}

		containerInfos = append(containerInfos, info)
	}

	// use tabwriter.NewWriter print container info on console
	// tab writer used to print line up
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	// the information column output by the console
	fmt.Fprint(w, "ID\tNAME\tPID\tSTATUS\tCOMMAND\tCREATED\n")
	for _, item := range containerInfos {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			item.ID,
			item.Name,
			item.PID,
			item.Status,
			item.Command,
			item.CreateTime,
		)
	}
	// flush the standard output stream buffer to print out the list of containers
	if err := w.Flush(); err != nil {
		log.Errorf("flush error %v.", err)
		return
	}
}

func getContainerInfo(file os.DirEntry) (*container.ContainerInfo, error) {
	var info container.ContainerInfo
	fileName := file.Name()
	cfgDir := fmt.Sprintf(container.DefaultLocation, fileName)
	cfgName := cfgDir + container.ConfigName
	data, err := os.ReadFile(cfgName)
	if err != nil {
		log.Errorf("read file %s error %v.", cfgName, err)
		return nil, err
	}
	err = sonic.Unmarshal(data, &info)
	if err != nil {
		log.Errorf("json unmarshal error %v.", err)
		return nil, err
	}

	return &info, nil
}

// isProcessRunning checks if a process with the given PID is still running
func isProcessRunning(pidStr string) bool {
	if pidStr == "" {
		return false
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		log.Errorf("Invalid PID format: %s", pidStr)
		return false
	}

	// Send signal 0 to check if process exists without affecting it
	if err := syscall.Kill(pid, 0); err != nil {
		// ESRCH means "No such process"
		if errors.Is(err, syscall.ESRCH) {
			return false
		}
		// Other errors might mean the process exists, but we don't have permission
		// In this case, we assume the process is running
		log.Warnf("Error checking process %d: %v", pid, err)
		return true
	}
	return true
}

// updateContainerStatusToExit updates container status to EXIT and clears PID
func updateContainerStatusToExit(containerName string) error {
	dirUrl := fmt.Sprintf(container.DefaultLocation, containerName)
	cfgFile := dirUrl + container.ConfigName

	// Read current container info
	content, err := os.ReadFile(cfgFile)
	if err != nil {
		return fmt.Errorf("read container config error: %v", err)
	}

	var info container.ContainerInfo
	if err := sonic.Unmarshal(content, &info); err != nil {
		return fmt.Errorf("unmarshal container info error: %v", err)
	}

	// Update status and clear PID
	info.Status = container.EXIT
	info.PID = ""

	// Write back to file
	newContent, err := sonic.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal container info error: %v", err)
	}

	if err := os.WriteFile(cfgFile, newContent, 0622); err != nil {
		return fmt.Errorf("write container config error: %v", err)
	}

	return nil
}
