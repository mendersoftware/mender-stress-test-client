// Copyright 2021 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package websocket

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/mendersoftware/go-lib-micro/ws"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 4 * time.Second
	// Maximum message size allowed from peer.
	maxMessageSize = 8192
	// Time allowed to read the next pong message from the peer.
	defaultPingWait = time.Minute
	// Default device connect path
	defaultDeviceConnectPath = "/api/devices/v1/deviceconnect/connect"
)

type Connection struct {
	writeMutex sync.Mutex
	// the connection handler
	connection *websocket.Conn
	// Time allowed to write a message to the peer.
	writeWait time.Duration
	// Maximum message size allowed from peer.
	maxMessageSize int64
	// Time allowed to read the next pong message from the peer.
	defaultPingWait time.Duration
	// Channel to stop the go routines
	done chan bool
}

// Websocket connection routine. setup the ping-pong and connection settings
func NewConnection(serverURL string, token string) (*Connection, error) {
	wsServerURL := "ws" + strings.TrimRight(serverURL[4:], "/")
	parsedURL, err := url.Parse(wsServerURL + defaultDeviceConnectPath)
	if err != nil {
		return nil, err
	}
	ctx := context.TODO()
	var wsconn *websocket.Conn

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	dialer := websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
		HTTPHeader: headers,
	}
	wsconn, resp, err := websocket.Dial(ctx, parsedURL.String(), &dialer)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	c := &Connection{
		connection:      wsconn,
		writeWait:       writeWait,
		maxMessageSize:  maxMessageSize,
		defaultPingWait: defaultPingWait,
		done:            make(chan bool),
	}
	wsconn.SetReadLimit(maxMessageSize)
	go c.pingPongHandler()

	return c, nil
}

func (c *Connection) pingPongHandler() {
	// handle the ping-pong connection health check
	/*
		err := c.connection.SetReadDeadline(time.Now().Add(c.defaultPingWait))
		if err != nil {
			return
		}
	*/
	//pingPeriod := (c.defaultPingWait * 9) / 10
	const pingPeriod = time.Hour
	rootCtx := context.Background()
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

Loop:
	for {
		select {
		case <-c.done:
			break Loop
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(rootCtx, time.Second*30)
			err := c.connection.Ping(ctx)
			cancel()
			if err != nil {
				_ = c.Close()
				break Loop
			}
		}
	}
}

func (c *Connection) GetWriteTimeout() time.Duration {
	return c.writeWait
}

func (c *Connection) WriteMessage(ctx context.Context, m *ws.ProtoMsg) (err error) {
	data, err := msgpack.Marshal(m)
	if err != nil {
		return err
	}
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return c.connection.Write(ctx, websocket.MessageBinary, data)
}

func (c *Connection) ReadMessage(ctx context.Context) (*ws.ProtoMsg, error) {
	_, data, err := c.connection.Read(ctx)
	if err != nil {
		return nil, err
	}

	m := &ws.ProtoMsg{}
	err = msgpack.Unmarshal(data, m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (c *Connection) Close() error {
	select {
	case c.done <- true:
	default:
	}
	return c.connection.CloseNow()
}
