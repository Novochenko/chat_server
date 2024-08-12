// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package chat

import (
	"bytes"
	"log"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// var upgrader = websocket.Upgrader{
// 	ReadBufferSize:  1024,
// 	WriteBufferSize: 1024,
// }

var (
	newline   = []byte{'\n'}
	space     = []byte{' '}
	counterID atomic.Uint64
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	ID       uint64
	Username string
	hub      *Hub
	// The websocket connection.
	conn *websocket.Conn
	// Buffered channel of outbound messages.
	send chan Message
}

func New(
	hub *Hub,
	conn *websocket.Conn,
	send chan Message,
) *Client {
	return &Client{hub: hub, conn: conn, send: make(chan Message), ID: newID()}
}
func (c *Client) ClientRegister() {
	c.hub.register <- c
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) ReadPump(messageRead chan struct{}) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		data = bytes.TrimSpace(bytes.Replace(data, newline, space, -1))
		log.Println(string(data))
		message := Message{
			Client: c,
			Body:   data,
		}
		messageRead <- struct{}{}
		<-messageRead
		c.hub.broadcast <- message
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write([]byte(message.Author + ": "))
			w.Write(message.Body)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				message := <-c.send
				w.Write(message.Body)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// // serveWs handles websocket requests from the peer.
// func (hub *Hub) ServeWs() http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		conn, err := upgrader.Upgrade(w, r, nil)
// 		if err != nil {
// 			log.Println(err)
// 			return
// 		}
// 		client := New(hub, conn, make(chan Message))
// 		s.sendMessageAuth()
// 		client.ClientRegister()

// 		// Allow collection of memory referenced by the caller by doing all work in
// 		// new goroutines.
// 		go client.WritePump()
// 		go client.ReadPump()
// 	}

// }

func newID() uint64 {
	counterID.Add(1)
	return counterID.Load()
}
func decrementID() {
	counterID.Add(0)
}
