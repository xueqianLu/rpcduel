package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

// IsIPCEndpoint reports whether the given URL targets a Unix-domain-socket
// JSON-RPC endpoint (geth-style ipc), e.g. unix:///tmp/geth.ipc.
func IsIPCEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, "unix")
}

// ipcSocketPath extracts the filesystem path from a unix:// URL.
func ipcSocketPath(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse endpoint: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "unix") {
		return "", fmt.Errorf("not a unix:// endpoint: %s", endpoint)
	}
	// unix:///abs/path → Path="/abs/path"; unix://./relative → Host=".", Path="/relative".
	path := u.Path
	if u.Host != "" && u.Host != "localhost" {
		path = u.Host + path
	}
	if path == "" {
		return "", fmt.Errorf("empty socket path in %s", endpoint)
	}
	return path, nil
}

// ipcConn is a single multiplexed Unix-socket JSON-RPC connection. The
// framing matches geth's IPC server: requests and responses are bare
// JSON values written back-to-back over the socket. Responses are
// demultiplexed by their JSON-RPC id.
type ipcConn struct {
	endpoint string
	opts     Options

	dialMu sync.Mutex
	conn   net.Conn
	dec    *json.Decoder

	pendMu  sync.Mutex
	pending map[int64]chan *Response
	closed  bool

	writeMu sync.Mutex
}

func newIPCConn(endpoint string, opts Options) *ipcConn {
	return &ipcConn{
		endpoint: endpoint,
		opts:     opts,
		pending:  make(map[int64]chan *Response),
	}
}

func (i *ipcConn) dial(ctx context.Context) (net.Conn, error) {
	i.dialMu.Lock()
	defer i.dialMu.Unlock()
	if i.conn != nil {
		return i.conn, nil
	}
	path, err := ipcSocketPath(i.endpoint)
	if err != nil {
		return nil, err
	}
	d := net.Dialer{Timeout: i.opts.Timeout}
	c, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("ipc dial: %w", err)
	}
	i.conn = c
	i.dec = json.NewDecoder(c)
	i.pendMu.Lock()
	i.closed = false
	i.pendMu.Unlock()
	go i.readPump(c, i.dec)
	return c, nil
}

func (i *ipcConn) readPump(c net.Conn, dec *json.Decoder) {
	for {
		var resp Response
		if err := dec.Decode(&resp); err != nil {
			i.fail(c, err)
			return
		}
		i.pendMu.Lock()
		ch, ok := i.pending[resp.ID]
		if ok {
			delete(i.pending, resp.ID)
		}
		i.pendMu.Unlock()
		if ok {
			ch <- &resp
		}
	}
}

func (i *ipcConn) fail(c net.Conn, _ error) {
	i.dialMu.Lock()
	if i.conn == c {
		i.conn = nil
		i.dec = nil
	}
	i.dialMu.Unlock()
	_ = c.Close()
	i.pendMu.Lock()
	pending := i.pending
	i.pending = make(map[int64]chan *Response)
	i.closed = true
	i.pendMu.Unlock()
	for _, ch := range pending {
		close(ch)
	}
}

func (i *ipcConn) call(ctx context.Context, body []byte, id int64) (*Response, error) {
	conn, err := i.dial(ctx)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Response, 1)
	i.pendMu.Lock()
	if i.closed {
		i.pendMu.Unlock()
		return nil, errors.New("ipc connection closed")
	}
	i.pending[id] = ch
	i.pendMu.Unlock()

	i.writeMu.Lock()
	if d, ok := ctx.Deadline(); ok {
		_ = conn.SetWriteDeadline(d)
	} else if i.opts.Timeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(i.opts.Timeout))
	}
	_, werr := conn.Write(body)
	i.writeMu.Unlock()
	if werr != nil {
		i.removePending(id)
		i.fail(conn, werr)
		return nil, fmt.Errorf("ipc write: %w", werr)
	}

	select {
	case <-ctx.Done():
		i.removePending(id)
		return nil, ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			return nil, errors.New("ipc connection lost")
		}
		if resp.Error != nil {
			return resp, resp.Error
		}
		return resp, nil
	}
}

func (i *ipcConn) removePending(id int64) {
	i.pendMu.Lock()
	delete(i.pending, id)
	i.pendMu.Unlock()
}

// Close terminates the underlying connection and any pending calls.
func (i *ipcConn) Close() {
	i.dialMu.Lock()
	c := i.conn
	i.conn = nil
	i.dialMu.Unlock()
	if c != nil {
		i.fail(c, errors.New("closed"))
	}
}
