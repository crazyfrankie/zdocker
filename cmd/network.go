package cmd

import (
	"errors"
	"fmt"
	
	"github.com/crazyfrank/zdocker/network"
	"github.com/spf13/cobra"
)

type createOption struct {
	driver string
	subnet string
}

func NewNetworkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "network [COMMAND]",
		Short:                 "container network commands",
		DisableFlagsInUseLine: true,
	}

	cmd.AddCommand(
		NewNetworkCreateCommand(),
		NewNetworkListCommand(),
		NewNetworkRemoveCommand(),
	)

	return cmd
}

func NewNetworkCreateCommand() *cobra.Command {
	var option createOption

	cmd := &cobra.Command{
		Use:   "create [OPTION]",
		Short: "create a container network",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("missing network name")
			}
			network.InitNetwork()
			err := network.CreateNetwork(option.driver, option.subnet, args[0])
			if err != nil {
				return fmt.Errorf("create network error %v", err)
			}

			return nil
		},
		DisableFlagsInUseLine: true,
	}

	flags := cmd.Flags()
	flags.StringVarP(&option.subnet, "subnet", "s", "", "subnet cidr")
	flags.StringVarP(&option.driver, "driver", "d", "", "network driver")

	return cmd
}

func NewNetworkListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list container network",
		RunE: func(cmd *cobra.Command, args []string) error {
			network.InitNetwork()
			network.ListNetwork()
			return nil
		},
		DisableFlagsInUseLine: true,
	}

	return cmd
}

func NewNetworkRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "remove container network",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("missing network name")
			}
			network.InitNetwork()
			err := network.RemoveNetwork(args[0])
			if err != nil {
				return fmt.Errorf("remove network error: %+v", err)
			}
			return nil
		},
		DisableFlagsInUseLine: true,
	}

	return cmd
}
