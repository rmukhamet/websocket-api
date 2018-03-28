// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/micro/go-micro/broker"
	"gitlab.com/TransportSystem/backend/websocket_service/message"
)

// Clients defines the set of registered clients
type Clients struct {
	sync.RWMutex
	clients map[string]*Client
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients Clients

	// Inbound messages from the clients.
	inbox chan *broker.Message

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		inbox:      make(chan *broker.Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients: Clients{
			clients: make(map[string]*Client),
		},
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.Register(client)
		case client := <-h.unregister:
			h.Unregister(client)
		case msg := <-h.inbox:
			m := &message.NatsMessage{}
			if err := json.Unmarshal(msg.Body, m); err != nil {
				log.Println("Hub.run.Unmarshal: ", err)
				continue
			}

			// Send the message to the specified client that has been identified by the token
			if m.Receiver != "" {
				go func(receiver string, message []byte) {
					if _, err := h.Client(receiver); err != nil {
						<-time.After(10 * time.Second)
						if _, err := h.Client(receiver); err != nil {
							log.Println(err)
							return
						}
					}
					client, _ := h.Client(receiver)
					select {
					case client.send <- message:
					default:
						h.Unregister(client)
					}
				}(m.Receiver, msg.Body)
				continue
			}

			// Send the message to the clients that have the specified person_id
			if personID, ok := msg.Header["person_id"]; ok {
				if _, err := uuid.Parse(personID); err != nil {
					log.Println("Hub.run error: malformed person ID in the header of the received message.")
					continue
				}
				h.TargetedSend(personID, msg.Body)
				continue
			}

			// Broadcast the message to the clients that have the specified owner
			owner, ok := msg.Header["owner"]
			if !ok {
				log.Println("Hub.run Error: no owner in the header of the received message.")
				continue
			}

			h.Broadcast(owner, msg.Body)
		}
	}
}

// Broadcast broadcasts the message to all registered clients
func (h *Hub) Broadcast(owner string, message []byte) {
	h.clients.RLock()
	for _, client := range h.clients.clients {
		if client.data.User.Owner == owner {
			select {
			case client.send <- message:
			default:
				h.clients.RUnlock()
				h.Unregister(client)
				h.clients.RLock()
			}
		}
	}
	h.clients.RUnlock()
}

// TargetedSend sends the message to the clients that have the specified person_id
func (h *Hub) TargetedSend(target string, message []byte) {
	h.clients.RLock()
	for _, client := range h.clients.clients {
		if client.data.User.Id == target {
			select {
			case client.send <- message:
			default:
				h.clients.RUnlock()
				h.Unregister(client)
				h.clients.RLock()
			}
		}
	}
	h.clients.RUnlock()
}

// Register registers new client
func (h *Hub) Register(client *Client) {
	h.clients.Lock()
	h.clients.clients[client.token] = client
	h.clients.Unlock()
	return
}

// Unregister unregisters the client
func (h *Hub) Unregister(client *Client) {
	h.clients.Lock()
	if _, ok := h.clients.clients[client.token]; ok {
		delete(h.clients.clients, client.token)
		close(client.send)
	}
	h.clients.Unlock()
	return
}

// Client returns the registered client by it's token
func (h *Hub) Client(token string) (*Client, error) {
	h.clients.RLock()
	defer h.clients.RUnlock()

	if client, ok := h.clients.clients[token]; ok {
		return client, nil
	}

	return nil, errors.New("Client is not registered: " + token)
}
