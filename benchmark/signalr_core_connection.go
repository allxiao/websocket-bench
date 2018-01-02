package benchmark

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ArieShout/websocket-bench/util"
	"github.com/gorilla/websocket"
	"github.com/teris-io/shortid"
)

var _ Subject = (*SignalrCoreConnection)(nil)

type SignalrCoreConnection struct {
	log  chan string
	host string

	counter *util.Counter

	sessions     []*Session
	sessionsLock sync.Mutex

	received chan MessageReceived
}

func (s *SignalrCoreConnection) Name() string {
	return "SignalR Core Connection"
}

func (s *SignalrCoreConnection) logError(id string, msg string, err error) {
	log.Printf("[Error][%s] %s due to %s", id, msg, err)
	s.counter.Stat("error", 1)
}

func (s *SignalrCoreConnection) logLatency(latency int64) {
	index := int(latency / 100)
	if index >= 10 {
		s.counter.Stat("latency:gt:1000", 1)
	} else {
		s.counter.Stat(fmt.Sprintf("latency:lt:%d00", index+1), 1)
	}
}

func (s *SignalrCoreConnection) Setup(config *Config) error {
	s.log = make(chan string)
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
			log.Printf("[Error][%s] %s due to %s", msgReceived.ClientID, "Failed to decode incoming message", err)
			s.counter.Stat("received:decode_error", 1)
			continue
		}

		if content.Type == 1 && content.Target == "echo" {
			sendStart, err := strconv.ParseInt(content.Arguments[1], 10, 64)
			if err != nil {
				log.Printf("[Error][%s] %s due to %s", msgReceived.ClientID, "Failed to decode start timestamp", err)
				s.counter.Stat("received:decode_error", 1)
				continue
			}
			s.logLatency((time.Now().UnixNano() - sendStart) / 1000000)
		}
	}
}

func (s *SignalrCoreConnection) LogChannel() chan string {
	return s.log
}

func (s *SignalrCoreConnection) Counters() map[string]int64 {
	return s.counter.Snapshot()
}

func (s *SignalrCoreConnection) newSession() (session *Session, err error) {
	defer func() {
		if err != nil {
			s.counter.Stat("connecting", -1)
			s.counter.Stat("connect:error", 1)
		}
	}()

	id, err := shortid.Generate()
	if err != nil {
		log.Println("ERROR: failed to generate uid due to", err)
		return
	}

	s.counter.Stat("connecting", 1)
	negotiateResponse, err := http.Post("http://"+s.host+"/chat/negotiate", "text/plain;charset=UTF-8", nil)
	if err != nil {
		s.logError(id, "Failed to negotiate with the server", err)
		return
	}
	defer negotiateResponse.Body.Close()

	decoder := json.NewDecoder(negotiateResponse.Body)
	var handshakeContent SignalRCoreHandshakeResp
	err = decoder.Decode(&handshakeContent)
	if err != nil {
		s.logError("", "Failed to decode connection id", err)
		return
	}

	wsURL := "ws://" + s.host + "/chat?id=" + handshakeContent.ConnectionId
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		s.logError(id, "Failed to connect to websocket", err)
		return nil, err
	}

	session = NewSession(id, s.received, s.counter, c)
	if session != nil {
		s.counter.Stat("connecting", -1)
		s.counter.Stat("connected", 1)
		return
	}

	err = fmt.Errorf("Nil session")
	return
}

func (s *SignalrCoreConnection) DoEnsureConnection(count int) error {
	if count < 0 {
		return nil
	}

	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	diff := count - len(s.sessions)
	if diff > 0 {
		for i := 0; i < diff; i++ {
			session, err := s.newSession()
			if err != nil {
				return err
			}
			session.Start()
			session.WriteTextMessage("{\"protocol\":\"json\"}\x1e")
			s.sessions = append(s.sessions, session)
		}
	} else {
		extra := s.sessions[count:]
		s.sessions = s.sessions[:count]
		for _, session := range extra {
			session.Close()
		}
	}

	return nil
}

func (s *SignalrCoreConnection) DoSend(clients int, intervalMillis int) error {
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

	for i := bound; i < sessionCount; i++ {
		s.sessions[i].RemoveMessageGenerator()
	}
	messageGen := &SignalRCoreTextMessageGenerator{
		interval: time.Millisecond * time.Duration(intervalMillis),
	}
	for i := 0; i < bound; i++ {
		s.sessions[i].InstallMessageGeneator(messageGen)
	}

	return nil
}

func (s *SignalrCoreConnection) DoStopSend() error {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	return s.doStopSendUnsafe()
}

func (s *SignalrCoreConnection) doStopSendUnsafe() error {
	for _, session := range s.sessions {
		session.RemoveMessageGenerator()
	}

	return nil
}
