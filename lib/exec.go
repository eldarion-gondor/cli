package gondorcli

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/flynn/flynn/pkg/attempt"
	"github.com/tj/go-spin"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/websocket"
)

type clientOpts struct {
	Tty    bool `json:"tty"`
	Width  int  `json:"width,omitempty"`
	Height int  `json:"height,omitempty"`
}

type remoteExec struct {
	endpoint      string
	enableTty     bool
	ws            *websocket.Conn
	httpClient    *http.Client
	tlsConfig     *tls.Config
	showAttaching bool
	callback      func(error)
	streamOpts    *streamOptions
	logger        *log.Logger
}

func (re *remoteExec) resetTimeout() {
	re.ws.SetDeadline(time.Now().Add(10 * time.Second))
}

func (re *remoteExec) execute() int {
	if re.callback == nil {
		re.callback = func(err error) {
			if err != nil {
				failure(err.Error())
			}
		}
	}
	done := make(chan struct{}, 1)
	var showIndicator bool
	var outs io.Writer
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		outs = os.Stdout
		showIndicator = true
	} else if terminal.IsTerminal(int(os.Stderr.Fd())) {
		outs = os.Stderr
		showIndicator = true
	}
	if re.showAttaching && showIndicator {
		s := spin.New()
		s.Set(spin.Box1)
		go func() {
		loop:
			for {
				select {
				case <-done:
					break loop
				case <-time.After(100 * time.Millisecond):
					fmt.Fprintf(outs, "\r\033[36mAttaching...\033[m %s ", s.Next())
				}
			}
		}()
	}
	httpClient := re.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	// wait for ok to report 200
	if err := (attempt.Strategy{
		Total: 2 * time.Minute,
		Delay: 1 * time.Second,
	}.Run(func() error {
		okURL := "http://" + re.endpoint + "/ok"
		resp, err := httpClient.Get(okURL)
		if err != nil {
			return err
		}
		if resp.StatusCode == 200 {
			return nil
		}
		return fmt.Errorf("http error %s", resp.Status)
	})); err != nil {
		done <- struct{}{}
		if re.showAttaching && showIndicator {
			fmt.Fprintf(outs, "\r\033[36mAttaching...\033[m failed\n")
		}
		re.callback(err)
		return 1
	}
	return func() int {
		opts := clientOpts{}
		if re.enableTty {
			if terminal.IsTerminal(int(os.Stdin.Fd())) {
				w, h, err := terminal.GetSize(int(os.Stdin.Fd()))
				if err != nil {
					fatal(err.Error())
				}
				state, err := terminal.MakeRaw(int(os.Stdin.Fd()))
				if err != nil {
					fatal(err.Error())
				}
				defer terminal.Restore(int(os.Stdin.Fd()), state)
				opts.Tty = true
				opts.Width = w
				opts.Height = h
			}
		}
		if err := (attempt.Strategy{
			Total: 10 * time.Second,
			Delay: 1 * time.Second,
		}.Run(func() error { return re.attach(opts) })); err != nil {
			if re.showAttaching && showIndicator {
				fmt.Fprintf(outs, "\r\033[36mAttaching...\033[m error\n")
			}
			re.callback(err)
			return 1
		}
		done <- struct{}{}
		if re.showAttaching && showIndicator {
			fmt.Fprintf(outs, "\r\033[36mAttaching...\033[m ok\n")
		}
		re.callback(nil)
		exitCode, err := re.interact()
		if err != nil {
			failure(err.Error())
			return 1
		}
		return exitCode
	}()
}

type streamOptions struct {
	stdin           bool
	stdout          bool
	stderr          bool
	tty             bool
	expectedStreams int
}

func newStreamOptions(opts clientOpts) *streamOptions {
	tty := opts.Tty
	var stdin, stdout, stderr bool
	if tty {
		stdin = true
		stdout = true
		stderr = false
	} else {
		stdin = true
		stdout = true
		stderr = true
	}
	expectedStreams := 1
	if stdin {
		expectedStreams++
	}
	if stdout {
		expectedStreams++
	}
	if stderr {
		expectedStreams++
	}
	return &streamOptions{
		stdin:           stdin,
		stdout:          stdout,
		stderr:          stderr,
		tty:             tty,
		expectedStreams: expectedStreams,
	}
}

func standardShellChannels(stdin, stdout, stderr bool) []ChannelType {
	// open three half-duplex channels
	channels := []ChannelType{WriteChannel, ReadChannel, ReadChannel}
	if !stdin {
		channels[0] = IgnoreChannel
	}
	if !stdout {
		channels[1] = IgnoreChannel
	}
	if !stderr {
		channels[2] = IgnoreChannel
	}
	return channels
}

func (re *remoteExec) attach(opts clientOpts) error {
	encoded, err := json.Marshal(opts)
	if err != nil {
		return err
	}
	config, err := websocket.NewConfig(fmt.Sprintf("ws://%s/", re.endpoint), "http://localhost")
	if err != nil {
		return err
	}
	config.TlsConfig = re.tlsConfig
	config.Header.Add("X-Pipe-Opts", string(encoded))
	ws, err := websocket.DialConfig(config)
	if err != nil {
		return err
	}
	re.ws = ws
	re.streamOpts = newStreamOptions(opts)
	return nil
}

type context struct {
	conn          *Conn
	stdinStream   io.WriteCloser
	stdoutStream  io.ReadCloser
	stderrStream  io.ReadCloser
	controlStream io.ReadWriteCloser
	tty           bool
}

func (re *remoteExec) createStreams() *context {
	sOpts := re.streamOpts
	channels := append(standardShellChannels(sOpts.stdin, sOpts.stdout, sOpts.stderr), ReadWriteChannel)
	conn := NewConn(re.logger, channels...)
	conn.SetIdleTimeout(10 * time.Second)
	streams := conn.Open(re.ws)
	return &context{
		conn:          conn,
		stdinStream:   streams[0],
		stdoutStream:  streams[1],
		stderrStream:  streams[2],
		controlStream: streams[3],
		tty:           sOpts.tty,
	}
}

func (re *remoteExec) interact() (int, error) {
	defer re.ws.Close()
	errch := make(chan error)
	ctx := re.createStreams()
	go func() {
		data, err := ioutil.ReadAll(ctx.controlStream)
		switch {
		case err != nil && err != io.EOF:
			errch <- fmt.Errorf("reading from control stream: %s", err)
		case len(data) > 0:
			errch <- fmt.Errorf("executing remote command: %s", data)
		default:
			errch <- nil
		}
		close(errch)
	}()
	go func() {
		defer ctx.stdinStream.Close()
		io.Copy(ctx.stdinStream, os.Stdin)
	}()
	go io.Copy(os.Stdout, ctx.stdoutStream)
	go io.Copy(os.Stderr, ctx.stderrStream)
	go ctx.conn.readLoop()
	for {
		select {
		case <-time.Tick(5 * time.Second):
			ctx.controlStream.Write([]byte{0x1})
		case err := <-errch:
			if err != nil {
				return 1, err
			}
			return 0, nil
		}
	}
}
