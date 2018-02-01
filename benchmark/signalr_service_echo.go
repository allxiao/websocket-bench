package benchmark

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/ArieShout/websocket-bench/util"
	"github.com/gorilla/websocket"
	"github.com/teris-io/shortid"
)

var _ Subject = (*SignalrServiceEcho)(nil)

type SignalrServiceEcho struct {
	host string

	counter *util.Counter

	sessions     []*Session
	sessionsLock sync.Mutex

	received chan MessageReceived
}

type SignalrServiceHandshake struct {
	ServiceUrl string `json:"serviceUrl"`
	JwtBearer  string `json:"jwtBearer"`
}

var httpPrefix = regexp.MustCompile("^https?://")

func (s *SignalrServiceEcho) Name() string {
	return "SignalR Service Echo"
}

func (s *SignalrServiceEcho) logError(errorGroup string, uid string, msg string, err error) {
	log.Printf("[Error][%s] %s due to %s", uid, msg, err)
	if errorGroup != "" {
		s.counter.Stat(errorGroup, 1)
	}
}

func (s *SignalrServiceEcho) logLatency(latency int64) {
	index := int(latency / 100)
	if index >= 10 {
		s.counter.Stat("message:gt:1000", 1)
	} else {
		s.counter.Stat(fmt.Sprintf("message:lt:%d00", index+1), 1)
	}
}

func (s *SignalrServiceEcho) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.processLatency()

	return nil
}

func (s *SignalrServiceEcho) processLatency() {
	for msgReceived := range s.received {
		var content SignalRCoreInvocation
		err := json.Unmarshal(msgReceived.Content[:len(msgReceived.Content)-1], &content)
		if err != nil {
			s.logError("message:decode_error", msgReceived.ClientID, "Failed to decode incoming message", err)
			continue
		}

		if content.Type == 1 && content.Target == "echo" {
			sendStart, err := strconv.ParseInt(content.Arguments[1], 10, 64)
			if err != nil {
				s.logError("message:decode_error", msgReceived.ClientID, "Failed to decode start timestamp", err)
				continue
			}
			s.logLatency((time.Now().UnixNano() - sendStart) / 1000000)
		}
	}
}

func (s *SignalrServiceEcho) Counters() map[string]int64 {
	return s.counter.Snapshot()
}

func (s *SignalrServiceEcho) NewSession() (session *Session, err error) {
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
		s.logError("connection:error", id, "Failed to negotiate with the server", err)
		return
	}
	defer negotiateResponse.Body.Close()

	decoder := json.NewDecoder(negotiateResponse.Body)
	var handshake SignalrServiceHandshake
	err = decoder.Decode(&handshake)
	if err != nil {
		s.logError("connection:error", id, "Failed to decode service URL and jwtBearer", err)
		return
	}

	baseURL := httpPrefix.ReplaceAllString(handshake.ServiceUrl, "ws://")
	wsURL := baseURL + "?uid=" + id + "&signalRTokenHeader=" + handshake.JwtBearer

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		s.logError("connection:error", id, "Failed to connect to websocket", err)
		return
	}

	session = NewSession(id, s.received, s.counter, c)
	if session != nil {
		s.counter.Stat("connection:inprogress", -1)
		s.counter.Stat("connection:established", 1)
		return
	}

	err = fmt.Errorf("Nil session")
	return
}

func (s *SignalrServiceEcho) DoEnsureConnection(count int, conPerSec int) error {
	if count < 0 {
		return nil
	}

	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	diff := count - len(s.sessions)
	if diff > 0 {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for _ = range ticker.C {
			nextBatch := diff
			if nextBatch > conPerSec {
				nextBatch = conPerSec
			}
			for i := 0; i < nextBatch; i++ {
				session, err := SessionBuilder(s).NewSession()
				if err != nil {
					return err
				}
				session.Start()
				session.WriteTextMessage("{\"protocol\":\"json\"}\x1e")
				s.sessions = append(s.sessions, session)
			}
			diff -= nextBatch
			if diff <= 0 {
				break
			}
		}
	} else {
		log.Printf("Reduce clients count from %d to %d", len(s.sessions), count)
		extra := s.sessions[count:]
		s.sessions = s.sessions[:count]
		for _, session := range extra {
			session.Close()
		}
	}

	return nil
}

func (s *SignalrServiceEcho) DoSend(clients int, intervalMillis int) error {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	if clients <= 0 {
		s.doStopSendUnsafe()
	}

	sessionCount := len(s.sessions)
	bound := sessionCount
	if clients < bound {
		bound = clients
	}

	s.doStopSendUnsafe()
	
	messageGen := &SignalRCoreTextMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
	}
	indices := rand.Perm(len(s.sessions))
	for i := 0; i < bound; i++ {
		s.sessions[indices[i]].InstallMessageGeneator(messageGen)
	}

	return nil
}

func (s *SignalrServiceEcho) DoStopSend() error {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	return s.doStopSendUnsafe()
}

func (s *SignalrServiceEcho) doStopSendUnsafe() error {
	for _, session := range s.sessions {
		session.RemoveMessageGenerator()
	}

	return nil
}

func (s *SignalrServiceEcho) DoClear(prefix string) error {
	s.counter.Clear(prefix)
	return nil
}
