package gondorcli

import (
	"io"
	"log"
	"time"

	"golang.org/x/net/websocket"
)

// ChannelType represents the type of channel.
type ChannelType int

const (
	// IgnoreChannel is an ignored channel.
	IgnoreChannel ChannelType = iota
	// ReadChannel only allows reads.
	ReadChannel
	// WriteChannel only allows writes.
	WriteChannel
	// ReadWriteChannel allows reads and writes.
	ReadWriteChannel
)

// Conn supports sending multiple binary channels over a websocket connection.
// Supports only the "channel.k8s.io" subprotocol.
type Conn struct {
	channels []*websocketChannel
	ready    chan struct{}
	ws       *websocket.Conn
	timeout  time.Duration
	logger   *log.Logger
}

// NewConn creates a WebSocket connection that supports a set of channels. Channels begin each
// web socket message with a single byte indicating the channel number (0-N). 255 is reserved for
// future use. The channel types for each channel are passed as an array, supporting the different
// duplex modes. Read and Write refer to whether the channel can be used as a Reader or Writer.
func NewConn(logger *log.Logger, channels ...ChannelType) *Conn {
	conn := &Conn{
		ready:    make(chan struct{}),
		channels: make([]*websocketChannel, len(channels)),
		logger:   logger,
	}
	for i := range conn.channels {
		switch channels[i] {
		case ReadChannel:
			conn.channels[i] = newWebsocketChannel(conn, byte(i), true, false)
		case WriteChannel:
			conn.channels[i] = newWebsocketChannel(conn, byte(i), false, true)
		case ReadWriteChannel:
			conn.channels[i] = newWebsocketChannel(conn, byte(i), true, true)
		case IgnoreChannel:
			conn.channels[i] = newWebsocketChannel(conn, byte(i), false, false)
		}
	}
	return conn
}

// SetIdleTimeout sets the interval for both reads and writes before timeout. If not specified,
// there is no timeout on the connection.
func (conn *Conn) SetIdleTimeout(duration time.Duration) {
	conn.timeout = duration
}

// Open the connection and create channels for reading and writing.
func (conn *Conn) Open(ws *websocket.Conn) []io.ReadWriteCloser {
	conn.initialize(ws)
	rwc := make([]io.ReadWriteCloser, len(conn.channels))
	for i := range conn.channels {
		rwc[i] = conn.channels[i]
	}
	return rwc
}

func (conn *Conn) initialize(ws *websocket.Conn) {
	conn.ws = ws
	close(conn.ready)
}

func (conn *Conn) resetTimeout() {
	if conn.timeout > 0 {
		conn.ws.SetDeadline(time.Now().Add(conn.timeout))
	}
}

func (conn *Conn) readLoop() error {
	defer conn.Close()
	for {
		conn.resetTimeout()
		var data []byte
		if err := websocket.Message.Receive(conn.ws, &data); err != nil {
			if err == io.EOF {
				conn.logger.Println("connection EOF")
				break
			}
			return err
		}
		if len(data) == 0 {
			continue
		}
		conn.logger.Printf("> %#v", data)
		channel := data[0]
		data = data[1:]
		if int(channel) >= len(conn.channels) {
			continue
		}
		if _, err := conn.channels[channel].DataFromSocket(data); err != nil {
			continue
		}
	}
	return nil
}

// Close is only valid after Open has been called
func (conn *Conn) Close() error {
	<-conn.ready
	for _, s := range conn.channels {
		s.Close()
	}
	conn.ws.Close()
	return nil
}

// write multiplexes the specified channel onto the websocket
func (conn *Conn) write(num byte, data []byte) (int, error) {
	conn.resetTimeout()
	frame := make([]byte, len(data)+1)
	frame[0] = num
	copy(frame[1:], data)
	if err := websocket.Message.Send(conn.ws, frame); err != nil {
		return 0, err
	}
	return len(data), nil
}

// websocketChannel represents a channel in a connection
type websocketChannel struct {
	conn *Conn
	num  byte
	r    io.Reader
	w    io.WriteCloser

	read, write bool
}

// newWebsocketChannel creates a pipe for writing to a websocket. Do not write to this pipe
// prior to the connection being opened. It may be no, half, or full duplex depending on
// read and write.
func newWebsocketChannel(conn *Conn, num byte, read, write bool) *websocketChannel {
	r, w := io.Pipe()
	return &websocketChannel{conn, num, r, w, read, write}
}

func (p *websocketChannel) Write(data []byte) (int, error) {
	if !p.write {
		// channel is not writable so let caller think we wrote data.
		return len(data), nil
	}
	return p.conn.write(p.num, data)
}

// DataFromSocket is invoked by the connection receiver to move data from the connection
// into a specific channel.
func (p *websocketChannel) DataFromSocket(data []byte) (int, error) {
	if !p.read {
		return len(data), nil
	}
	return p.w.Write(data)
}

func (p *websocketChannel) Read(data []byte) (int, error) {
	if !p.read {
		return 0, io.EOF
	}
	return p.r.Read(data)
}

func (p *websocketChannel) Close() error {
	p.conn.logger.Printf("closing channel %d", p.num)
	p.conn.write(p.num, []byte{})
	return p.w.Close()
}
