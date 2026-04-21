// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// IsWebSocketEndpoint reports whether the given URL uses a WebSocket scheme.
func IsWebSocketEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	s := strings.ToLower(u.Scheme)
	return s == "ws" || s == "wss"
}

// wsConn is a single multiplexed WebSocket connection to a JSON-RPC node.
//
// It supports concurrent Call invocations: requests are written under a
// mutex and a single read pump goroutine demultiplexes responses by JSON-RPC
// id into a per-request channel. Connections are dialed lazily and
// transparently re-dialed on failure.
type wsConn struct {
	endpoint string
	opts     Options

	dialMu sync.Mutex
	conn   *websocket.Conn

	pendMu  sync.Mutex
	pending map[int64]chan *Response
	closed  bool

	writeMu sync.Mutex
}

func newWSConn(endpoint string, opts Options) *wsConn {
	return &wsConn{
		endpoint: endpoint,
		opts:     opts,
		pending:  make(map[int64]chan *Response),
	}
}

func (w *wsConn) dial(ctx context.Context) (*websocket.Conn, error) {
	w.dialMu.Lock()
	defer w.dialMu.Unlock()
	if w.conn != nil {
		return w.conn, nil
	}

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = w.opts.Timeout
	if w.opts.InsecureSkipVerify {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in dev flag
	}

	hdr := http.Header{}
	if w.opts.UserAgent != "" {
		hdr.Set("User-Agent", w.opts.UserAgent)
	}
	for k, v := range w.opts.Headers {
		hdr.Set(k, v)
	}

	c, resp, err := dialer.DialContext(ctx, w.endpoint, hdr)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("ws dial: %w", err)
	}
	w.conn = c
	w.pendMu.Lock()
	w.closed = false
	w.pendMu.Unlock()
	go w.readPump(c)
	return c, nil
}

// readPump is the only goroutine that reads from the connection. It exits
// when the connection is closed and notifies any in-flight requests by
// dropping their channels (closing the channel makes Call return an error).
func (w *wsConn) readPump(c *websocket.Conn) {
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			w.fail(c, err)
			return
		}
		var resp Response
		if err := json.Unmarshal(data, &resp); err != nil {
			// Malformed frame: ignore and keep reading.
			continue
		}
		w.pendMu.Lock()
		ch, ok := w.pending[resp.ID]
		if ok {
			delete(w.pending, resp.ID)
		}
		w.pendMu.Unlock()
		if ok {
			ch <- &resp
		}
	}
}

func (w *wsConn) fail(c *websocket.Conn, _ error) {
	w.dialMu.Lock()
	if w.conn == c {
		w.conn = nil
	}
	w.dialMu.Unlock()
	_ = c.Close()
	w.pendMu.Lock()
	pending := w.pending
	w.pending = make(map[int64]chan *Response)
	w.closed = true
	w.pendMu.Unlock()
	for _, ch := range pending {
		close(ch)
	}
}

func (w *wsConn) call(ctx context.Context, body []byte, id int64) (*Response, error) {
	conn, err := w.dial(ctx)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Response, 1)
	w.pendMu.Lock()
	if w.closed {
		w.pendMu.Unlock()
		return nil, errors.New("ws connection closed")
	}
	w.pending[id] = ch
	w.pendMu.Unlock()

	w.writeMu.Lock()
	if d, ok := ctx.Deadline(); ok {
		_ = conn.SetWriteDeadline(d)
	} else if w.opts.Timeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(w.opts.Timeout))
	}
	werr := conn.WriteMessage(websocket.TextMessage, body)
	w.writeMu.Unlock()
	if werr != nil {
		w.removePending(id)
		w.fail(conn, werr)
		return nil, fmt.Errorf("ws write: %w", werr)
	}

	select {
	case <-ctx.Done():
		w.removePending(id)
		return nil, ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			return nil, errors.New("ws connection lost")
		}
		if resp.Error != nil {
			return resp, resp.Error
		}
		return resp, nil
	}
}

func (w *wsConn) removePending(id int64) {
	w.pendMu.Lock()
	delete(w.pending, id)
	w.pendMu.Unlock()
}

// Close terminates the underlying connection and any pending calls.
func (w *wsConn) Close() {
	w.dialMu.Lock()
	c := w.conn
	w.conn = nil
	w.dialMu.Unlock()
	if c != nil {
		w.fail(c, errors.New("closed"))
	}
}
