// Package message defines struct for NATS messages that the service expect to come
package message

// NatsMessage is a struct for messages the service send to the front-end
type NatsMessage struct {
	Type     string `json:"type"`
	Header   string `json:"title"`
	Body     string `json:"content"`
	Sender   string `json:"from,omitempty"`
	Receiver string `json:"to,omitempty"`
}
