package cmd

import (
	"fmt"
	"os"
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
