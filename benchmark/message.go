package benchmark

import (
	"time"

	"github.com/gorilla/websocket"
)

// Message represents a WebSocket message with the type and the payload bytes.
type Message interface {
	Type() int
	Bytes() []byte
}

// MessageReceived is a wrapper for the frame(s) received from SignalR service WebSocket connection.
type MessageReceived struct {
	ClientID string
	Content  []byte
}

// PlainMessage is the normal content based WebSocket message.
type PlainMessage struct {
	tpe          int
	messageBytes []byte
}

// Type returns the type of the PlainMessage.
func (msg PlainMessage) Type() int {
	return msg.tpe
}

// Bytes returns the content bytes.
func (msg PlainMessage) Bytes() []byte {
	return msg.messageBytes
}

// CloseMessage is the close WebSocket connection message.
type CloseMessage struct {
}

// Type returns the websocket.CloseMessage.
func (c CloseMessage) Type() int {
	return websocket.CloseMessage
}

// Bytes returns the control bytes for the close action.
func (c CloseMessage) Bytes() []byte {
	return websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
}

// MessageGenerator is the interface to generate WebSocket messages at the given interval.
type MessageGenerator interface {
	Interval() time.Duration
	Generate(uid string, invocationID int64) Message
}
