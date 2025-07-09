package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/crazyfrankie/zdocker/container"
)

func NewInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Init container process",
		Long:  "Init container process run user's process in container . Do not call it outside",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Infof("init come on")

			return container.RunContainerInitProcess()
		},
		DisableFlagsInUseLine: true,
	}

	return cmd
}
