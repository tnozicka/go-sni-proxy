package sniproxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/tnozicka/go-sni-proxy/pkg/genericclioptions"
	"github.com/tnozicka/go-sni-proxy/pkg/signals"
	"github.com/tnozicka/go-sni-proxy/pkg/version"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
)

type IdentityProxyOptions struct {
	ListenAddr string
	TargetPort uint16
}

func NewIdentityProxyOptions(streams genericclioptions.IOStreams) *IdentityProxyOptions {
	return &IdentityProxyOptions{
		ListenAddr: ":5000",
		TargetPort: 443,
	}
}

func NewIdentityProxyCmd(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewIdentityProxyOptions(streams)

	cmd := &cobra.Command{
		Use:   "identity-proxy",
		Short: "Runs SNI proxy that sends the traffic to the original destination.",
		Long:  "Runs SNI proxy that sends the traffic to the original destination.",
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

	cmd.Flags().StringVarP(&o.ListenAddr, "listen", "", o.ListenAddr, "Address to listen on.")
	cmd.Flags().Uint16VarP(&o.TargetPort, "target-port", "", o.TargetPort, "Destination port for connecting to the backend.")

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

	l, err := net.Listen("tcp", o.ListenAddr)
	if err != nil {
		klog.Fatal(err)
	}
	klog.Info("Listening on %s", l.Addr())

	for {
		select {
		case <-ctx.Done():
			klog.InfoS("Closing listener")
			return l.Close()

		default:
			conn, err := l.Accept()
			if err != nil {
				klog.ErrorS(err, "Can't accept a connection")
				continue
			}

			go handleConnection(conn, o.TargetPort)
		}
	}
}

type readOnlyConn struct {
	reader io.Reader
}

func (conn readOnlyConn) Read(p []byte) (int, error)         { return conn.reader.Read(p) }
func (conn readOnlyConn) Write(p []byte) (int, error)        { return 0, io.ErrClosedPipe }
func (conn readOnlyConn) Close() error                       { return nil }
func (conn readOnlyConn) LocalAddr() net.Addr                { return nil }
func (conn readOnlyConn) RemoteAddr() net.Addr               { return nil }
func (conn readOnlyConn) SetDeadline(t time.Time) error      { return nil }
func (conn readOnlyConn) SetReadDeadline(t time.Time) error  { return nil }
func (conn readOnlyConn) SetWriteDeadline(t time.Time) error { return nil }

func readClientHello(reader io.Reader) (*tls.ClientHelloInfo, error) {
	var hello *tls.ClientHelloInfo

	err := tls.Server(readOnlyConn{reader: reader}, &tls.Config{
		GetConfigForClient: func(argHello *tls.ClientHelloInfo) (*tls.Config, error) {
			hello = new(tls.ClientHelloInfo)
			*hello = *argHello
			return nil, nil
		},
	}).Handshake()

	if hello == nil {
		return nil, err
	}

	return hello, nil
}

// func peekClientHello(reader io.Reader) (*tls.ClientHelloInfo, io.Reader, error) {
// 	peekedBytes := new(bytes.Buffer)
// 	hello, err := readClientHello(io.TeeReader(reader, peekedBytes))
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	return hello, io.MultiReader(peekedBytes, reader), nil
// }

func handleConnection(clientConn net.Conn, targetPort uint16) {
	defer clientConn.Close()

	err := clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		klog.ErrorS(err, "Can't set read deadline")
		return
	}

	peekedBytes := bytes.Buffer{}
	clientHello, err := readClientHello(io.TeeReader(clientConn, &peekedBytes))
	if err != nil {
		klog.ErrorS(err, "Can't read ClientHello")
		return
	}

	if err := clientConn.SetReadDeadline(time.Time{}); err != nil {
		klog.ErrorS(err, "Can't set read deadline")
		return
	}

	backendConn, err := net.DialTimeout("tcp", net.JoinHostPort(clientHello.ServerName, strconv.FormatUint(uint64(targetPort), 10)), 5*time.Second)
	if err != nil {
		klog.ErrorS(err, "Can't dial backend")
		return
	}
	defer backendConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// const bufferSize int = 64 * 1024 * 1024

	go func() {
		// buffer := make([]byte, bufferSize)
		// io.CopyBuffer(backendConn, &peekedBytes, buffer)
		// io.CopyBuffer(backendConn, clientConn, buffer)
		io.Copy(backendConn, &peekedBytes)
		io.Copy(backendConn, clientConn)
		backendConn.(*net.TCPConn).CloseWrite()
		wg.Done()
	}()

	go func() {
		// buffer := make([]byte, bufferSize)
		// io.CopyBuffer(clientConn, backendConn, buffer)
		io.Copy(clientConn, backendConn)
		clientConn.(*net.TCPConn).CloseWrite()
		wg.Done()
	}()

	wg.Wait()
}
