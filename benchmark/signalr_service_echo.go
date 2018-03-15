package benchmark

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/ArieShout/websocket-bench/util"
	"github.com/gorilla/websocket"
	"github.com/teris-io/shortid"
)

var _ Subject = (*SignalrServiceEcho)(nil)

type SignalrServiceEcho struct {
	WithCounter
	WithSessions
}

type SignalrServiceHandshake struct {
	ServiceUrl string `json:"serviceUrl"`
	JwtBearer  string `json:"jwtBearer"`
}

var httpPrefix = regexp.MustCompile("^https?://")

func (s *SignalrServiceEcho) Name() string {
	return "SignalR Service Echo"
}

func (s *SignalrServiceEcho) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.processLatency("echo")

	return nil
}

func (s *SignalrServiceEcho) processLatency(target string) {
	for msgReceived := range s.received {
		var content SignalRCoreInvocation
		err := json.Unmarshal(msgReceived.Content[:len(msgReceived.Content)-1], &content)
		if err != nil {
			s.LogError("message:decode_error", msgReceived.ClientID, "Failed to decode incoming message", err)
			continue
		}

		if content.Type == 1 && content.Target == target {
			sendStart, err := strconv.ParseInt(content.Arguments[1], 10, 64)
			if err != nil {
				s.LogError("message:decode_error", msgReceived.ClientID, "Failed to decode start timestamp", err)
				continue
			}
			s.LogLatency((time.Now().UnixNano() - sendStart) / 1000000)
		}
	}
}

func (s *SignalrServiceEcho) newSession() (session *Session, err error) {
	defer func() {
		if err != nil {
			s.counter.Stat("connection:inprogress", -1)
			s.counter.Stat("connection:error", 1)
		}
	}()

	s.counter.Stat("connection:inprogress", 1)

	id, err := shortid.Generate()
	if err != nil {
		log.Println("ERROR: failed to generate uid due to", err)
		return
	}

	negotiateResponse, err := http.Get("http://" + s.host + "/chat")
	if err != nil {
		s.LogError("connection:error", id, "Failed to negotiate with the server", err)
		return
	}
	defer negotiateResponse.Body.Close()

	decoder := json.NewDecoder(negotiateResponse.Body)
	var handshake SignalrServiceHandshake
	err = decoder.Decode(&handshake)
	if err != nil {
		s.LogError("connection:error", id, "Failed to decode service URL and jwtBearer", err)
		return
	}

	baseURL := httpPrefix.ReplaceAllString(handshake.ServiceUrl, "ws://")
	wsURL := baseURL + "?uid=" + id + "&signalRTokenHeader=" + handshake.JwtBearer

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		s.LogError("connection:error", id, "Failed to connect to websocket", err)
		return
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

func (s *SignalrServiceEcho) DoEnsureConnection(count int, conPerSec int) error {
	return s.doEnsureConnection(count, conPerSec, func(withSessions *WithSessions) (*Session, error) {
		return s.newSession()
	})
}

func (s *SignalrServiceEcho) DoSend(clients int, intervalMillis int) error {
	return s.doSend(clients, intervalMillis, &SignalRCoreTextMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
		Target: "echo",
	})
}
