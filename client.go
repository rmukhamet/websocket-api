// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/micro/go-micro/client"
	"github.com/tomasen/realip"
	api "gitlab.com/TransportSystem/backend/api_service/proto/apiservice"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

var newline = []byte{'\n'}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// TokenAuth is a jwt that should be send the very first from a Client to authenticate
type TokenAuth struct {
	Token string `json:"token,omitempty"`
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// Client's details
	data *api.DecodeRes

	// Client's JWT
	token string

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// authenticate check Client connections for allowability
func (c *Client) authenticate(clientIP string) {
	var (
		err     error
		message []byte
	)
	defer func() {
		if err != nil {
			c.conn.Close()
		}
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	_, message, err = c.conn.ReadMessage()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
			log.Printf("error: %v", err)
		}
		return
	}
	token := &TokenAuth{}
	err = json.Unmarshal(message, token)
	if err != nil {
		log.Println("Received message: ", message)
		log.Println("Unmarshal: ", err)
		c.conn.WriteMessage(websocket.TextMessage, []byte("{\"error\":\"Malformed message: a token had been expected\"}"))
		return
	}
	req := &api.DecodeReq{
		Address: clientIP,
		Jwt:     token.Token,
	}
	apiReq := client.NewRequest(api.DefaultServiceName, "Apiservice.Decode", req)
	apiRsp := &api.DecodeRes{}
	if err = client.Call(context.TODO(), apiReq, apiRsp); err != nil {
		log.Println("Authorization Error: ", err)
		e := strings.Replace(err.Error(), "\"", "\\\"", -1)
		c.conn.WriteMessage(websocket.TextMessage, []byte("{\"error\":\"Authorization Error: "+e+"\"}"))
		return
	}

	c.data, c.token = apiRsp, token.Token
	c.hub.register <- c
	go c.readPump()
	go c.writePump()
}

// readPump pumps messages from the websocket connection for the further processing.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		// TODO: Incoming Message processing
		log.Printf("Message received. Lenght: %v", len(message))
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
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
				log.Println("NextWriter: ", err)
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				log.Println("Close: ", err)
				return
			}

			// Send buffered messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w, err := c.conn.NextWriter(websocket.TextMessage)
				if err != nil {
					log.Println("NextWriter: ", err)
					return
				}
				w.Write(<-c.send)
				if err := w.Close(); err != nil {
					log.Println("Close: ", err)
					return
				}
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Println("WriteMessage: ", err)
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade: ", err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}

	clientIP := realip.RealIP(r)
	client.authenticate(clientIP)
}
