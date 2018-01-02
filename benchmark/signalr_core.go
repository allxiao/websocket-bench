package benchmark

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const SignalRMessageTerminator = '\x1e'

type SignalRCoreHandshakeResp struct {
	AvailableTransports []string `json:"availableTransports"`
	ConnectionId        string   `json:"connectionId"`
}

type SignalRCoreInvocation struct {
	InvocationId string   `json:"invocationId"`
	Type         int      `json:"type"`
	Target       string   `json:"target"`
	NonBlocking  bool     `json:"nonBlocking"`
	Arguments    []string `json:"arguments"`
}

type SignalRCoreTextMessageGenerator struct {
	uid          string
	interval     time.Duration
	invocationId int
}

var _ MessageGenerator = (*SignalRCoreTextMessageGenerator)(nil)

func (g *SignalRCoreTextMessageGenerator) Interval() time.Duration {
	return g.interval
}

func (g *SignalRCoreTextMessageGenerator) Generate() Message {
	g.invocationId++
	msg, err := json.Marshal(&SignalRCoreInvocation{
		Type:         1,
		InvocationId: strconv.Itoa(g.invocationId),
		Target:       "echo",
		Arguments: []string{
			g.uid,
			strconv.FormatInt(time.Now().UnixNano(), 10),
		},
		NonBlocking: false,
	})
	if err != nil {
		log.Println("ERROR: failed to encoding SignalR message", err)
		return nil
	}
	msg = append(msg, SignalRMessageTerminator)
	return PlainMessage{websocket.TextMessage, msg}
}

func (g *SignalRCoreTextMessageGenerator) SetUid(uid string) {
	g.uid = uid
}
