package sniproxy

import (
	"github.com/spf13/cobra"
	"github.com/tnozicka/go-sni-proxy/pkg/cmdutil"
	"github.com/tnozicka/go-sni-proxy/pkg/genericclioptions"
)

func NewSNIProxyCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.ReadFlagsFromEnv("SNI_PROXY_", cmd)
		},
	}

	cmd.AddCommand(NewIdentityProxyCmd(streams))

	// TODO: wrap help func for the root command and every subcommand to add a line about automatic env vars and the prefix.

	cmdutil.InstallKlog(cmd)

	return cmd
}
