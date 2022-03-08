package sniproxy

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/tnozicka/go-sni-proxy/pkg/genericclioptions"
	"github.com/tnozicka/go-sni-proxy/pkg/signals"
	"github.com/tnozicka/go-sni-proxy/pkg/version"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type IdentityProxyOptions struct {
}

func NewIdentityProxyOptions(streams genericclioptions.IOStreams) *IdentityProxyOptions {
	return &IdentityProxyOptions{}
}

func NewOperatorCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewIdentityProxyOptions(streams)

	cmd := &cobra.Command{
		Use:   "operator",
		Short: "Run the scylla operator.",
		Long:  `Run the scylla operator.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := o.Validate()
			if err != nil {
				return err
			}

			err = o.Complete()
			if err != nil {
				return err
			}

			err = o.Run(streams, cmd)
			if err != nil {
				return err
			}

			return nil
		},

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	return cmd
}

func (o *IdentityProxyOptions) Validate() error {
	var errs []error

	return utilerrors.NewAggregate(errs)
}

func (o *IdentityProxyOptions) Complete() error {
	return nil
}

func (o *IdentityProxyOptions) Run(streams genericclioptions.IOStreams, cmd *cobra.Command) error {
	klog.Infof("%s version %s", cmd.Name(), version.Get())
	cliflag.PrintFlags(cmd.Flags())

	stopCh := signals.StopChannel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		cancel()
	}()

	<-ctx.Done()

	return nil
}
