package benchmark

import (
	"time"

	"github.com/gorilla/websocket"
)

type Message interface {
	Type() int
	Bytes() []byte
}

type MessageReceived struct {
	ClientID string
	Content  []byte
}

type PlainMessage struct {
	tpe          int
	messageBytes []byte
}

func (msg PlainMessage) Type() int {
	return msg.tpe
}

func (msg PlainMessage) Bytes() []byte {
	return msg.messageBytes
}

type CloseMessage struct {
}

func (c CloseMessage) Type() int {
	return websocket.CloseMessage
}

func (c CloseMessage) Bytes() []byte {
	return websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
}

type MessageGenerator interface {
	Interval() time.Duration
	Generate(uid string, invocationId int64) Message
}
