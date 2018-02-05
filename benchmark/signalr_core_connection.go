package benchmark

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ArieShout/websocket-bench/util"
	"github.com/gorilla/websocket"
	"github.com/teris-io/shortid"
)

var _ Subject = (*SignalrCoreConnection)(nil)

type SignalrCoreConnection struct {
	WithCounter

	WithSessions
}

func (s *SignalrCoreConnection) Name() string {
	return "SignalR Core Connection"
}

func (s *SignalrCoreConnection) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.processLatency()

	return nil
}

func (s *SignalrCoreConnection) processLatency() {
	for msgReceived := range s.received {
		var content SignalRCoreInvocation
		err := json.Unmarshal(msgReceived.Content[:len(msgReceived.Content)-1], &content)
		if err != nil {
			s.LogError("message:decode_error", msgReceived.ClientID, "Failed to decode incoming message", err)
			continue
		}

		if content.Type == 1 && content.Target == "echo" {
			sendStart, err := strconv.ParseInt(content.Arguments[1], 10, 64)
			if err != nil {
				s.LogError("message:decode_error", msgReceived.ClientID, "Failed to decode start timestamp", err)
				continue
			}
			s.LogLatency((time.Now().UnixNano() - sendStart) / 1000000)
		}
	}
}

func (s *SignalrCoreConnection) newSession() (session *Session, err error) {
	defer func() {
		if err != nil {
			s.counter.Stat("connection:inprogress", -1)
			s.counter.Stat("connection:error", 1)
		}
	}()

	id, err := shortid.Generate()
	if err != nil {
		log.Println("ERROR: failed to generate uid due to", err)
		return
	}

	s.counter.Stat("connection:inprogress", 1)
	negotiateResponse, err := http.Post("http://"+s.host+"/chat/negotiate", "text/plain;charset=UTF-8", nil)
	if err != nil {
		s.LogError("connection:error", id, "Failed to negotiate with the server", err)
		return
	}
	defer negotiateResponse.Body.Close()

	decoder := json.NewDecoder(negotiateResponse.Body)
	var handshakeContent SignalRCoreHandshakeResp
	err = decoder.Decode(&handshakeContent)
	if err != nil {
		s.LogError("connection:error", id, "Failed to decode connection id", err)
		return
	}

	wsURL := "ws://" + s.host + "/chat?id=" + handshakeContent.ConnectionId
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		s.LogError("connection:error", id, "Failed to connect to websocket", err)
		return nil, err
	}

	session = NewSession(id, s.received, s.counter, c)
	if session != nil {
		s.counter.Stat("connection:inprogress", -1)
		s.counter.Stat("connection:established", 1)

		session.Start()
		session.WriteTextMessage("{\"protocol\":\"json\"}\x1e")
		return
	}

	err = fmt.Errorf("Nil session")
	return
}

func (s *SignalrCoreConnection) DoEnsureConnection(count int, conPerSec int) error {
	return s.doEnsureConnection(count, conPerSec, func(withSessions *WithSessions) (*Session, error) {
		return s.newSession()
	})
}

func (s *WithSessions) DoSend(clients int, intervalMillis int) error {
	return s.doSend(clients, intervalMillis, &SignalRCoreTextMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
	})
}
