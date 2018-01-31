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
	"github.com/vmihailenco/msgpack"
)

var _ Subject = (*SignalrCoreMsgpack)(nil)

type SignalrCoreMsgpack struct {
	host string

	counter *util.Counter

	sessions     []*Session
	sessionsLock sync.Mutex

	received chan MessageReceived
}

func (s *SignalrCoreMsgpack) Name() string {
	return "SignalR Core MessagePack"
}

func (s *SignalrCoreMsgpack) logError(errorGroup string, uid string, msg string, err error) {
	log.Printf("[Error][%s] %s due to %s", uid, msg, err)
	if errorGroup != "" {
		s.counter.Stat(errorGroup, 1)
	}
}

func (s *SignalrCoreMsgpack) logLatency(latency int64) {
	index := int(latency / 100)
	if index >= 10 {
		s.counter.Stat("message:gt:1000", 1)
	} else {
		s.counter.Stat(fmt.Sprintf("message:lt:%d00", index+1), 1)
	}
}

func (s *SignalrCoreMsgpack) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.processLatency()

	return nil
}

var numBitsToShift = []uint{0, 7, 14, 21, 28}

func parseMessage(bytes []byte) ([]byte, error) {
	moreBytes := true
	msgLen := 0
	numBytes := 0
	for moreBytes && numBytes < len(bytes) {
		byteRead := bytes[numBytes]
		msgLen = msgLen | int(uint(byteRead&0x7F)<<numBitsToShift[numBytes])
		numBytes++
		moreBytes = (byteRead & 0x80) != 0
	}

	if msgLen+numBytes > len(bytes) {
		return nil, fmt.Errorf("Not enough data in message, message length = %d, length section bytes = %d, data length = %d", msgLen, numBytes, len(bytes))
	}

	return bytes[numBytes : numBytes+msgLen], nil
}

func (s *SignalrCoreMsgpack) processLatency() {
	for msgReceived := range s.received {
		msg, err := parseMessage(msgReceived.Content)
		if err != nil {
			s.logError("message:decode_error", msgReceived.ClientID, "Failed to parse incoming message", err)
			continue
		}
		var content MsgpackInvocation
		err = msgpack.Unmarshal(msg, &content)
		if err != nil {
			s.logError("message:decode_error", msgReceived.ClientID, "Failed to decode incoming message", err)
			continue
		}

		if content.Target == "echo" {
			sendStart, err := strconv.ParseInt(content.Params[1], 10, 64)
			if err != nil {
				s.logError("message:decode_error", msgReceived.ClientID, "Failed to decode start timestamp", err)
				continue
			}
			s.logLatency((time.Now().UnixNano() - sendStart) / 1000000)
		}
	}
}

func (s *SignalrCoreMsgpack) Counters() map[string]int64 {
	return s.counter.Snapshot()
}

func (s *SignalrCoreMsgpack) NewSession() (session *Session, err error) {
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
		s.logError("connection:error", id, "Failed to negotiate with the server", err)
		return
	}
	defer negotiateResponse.Body.Close()

	decoder := json.NewDecoder(negotiateResponse.Body)
	var handshakeContent SignalRCoreHandshakeResp
	err = decoder.Decode(&handshakeContent)
	if err != nil {
		s.logError("connection:error", id, "Failed to decode connection id", err)
		return
	}

	wsURL := "ws://" + s.host + "/chat?id=" + handshakeContent.ConnectionId
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		s.logError("connection:error", id, "Failed to connect to websocket", err)
		return nil, err
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

func (s *SignalrCoreMsgpack) DoEnsureConnection(count int, conPerSec int) error {
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
			log.Printf("Spawn %d clients, current clients count %d", nextBatch, len(s.sessions))
			for i := 0; i < nextBatch; i++ {
				session, err := SessionBuilder(s).NewSession()
				if err != nil {
					return err
				}
				session.Start()
				session.WriteTextMessage("{\"protocol\":\"messagepack\"}\x1e")
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

func (s *SignalrCoreMsgpack) DoSend(clients int, intervalMillis int) error {
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

	for i := sessionCount - bound - 1; i >= 0; i-- {
		s.sessions[i].RemoveMessageGenerator()
	}
	messageGen := &MessagePackMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
	}
	for i := sessionCount - bound; i < sessionCount; i++ {
		s.sessions[i].InstallMessageGeneator(messageGen)
	}

	return nil
}

func (s *SignalrCoreMsgpack) DoStopSend() error {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	return s.doStopSendUnsafe()
}

func (s *SignalrCoreMsgpack) doStopSendUnsafe() error {
	for _, session := range s.sessions {
		session.RemoveMessageGenerator()
	}

	return nil
}

func (s *SignalrCoreMsgpack) DoClear(prefix string) error {
	s.counter.Clear(prefix)
	return nil
}
