package benchmark

import (
	"log"
	"sync"
	"time"

	"github.com/ArieShout/websocket-bench/util"
	"github.com/gorilla/websocket"
)

// Config defines the basic configuration for the benchmark.
type Config struct {
	Host    string
	Subject string

	ConnectionPerSecond int
}

// Subject defines the interface for a test subject.
type Subject interface {
	Name() string
	Setup(config *Config) error
	LogChannel() chan string
	Counters() map[string]int64

	DoEnsureConnection(count int) error
}

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
	Generate() Message
	SetUid(uid string)
}

// Session represents a single connection to the given websocket host.
type Session struct {
	ID       string
	Conn     *websocket.Conn
	Control  chan string
	Sending  chan Message
	received chan MessageReceived
	States   chan string

	counter *util.Counter

	genLock   sync.Mutex
	genClose  chan struct{}
	generator MessageGenerator
}

func NewSession(id string, received chan MessageReceived, counter *util.Counter, conn *websocket.Conn) *Session {
	s := new(Session)
	s.ID = id
	s.counter = counter
	s.Conn = conn
	s.Control = make(chan string)
	s.Sending = make(chan Message)
	s.received = received
	s.States = make(chan string)
	s.genLock = sync.Mutex{}

	return s
}

func (s *Session) Start() {
	go s.sendingWorker()
	go s.receivedWorker(s.ID)
}

func (s *Session) WriteTextMessage(msg string) {
	s.Sending <- PlainMessage{
		tpe:          websocket.TextMessage,
		messageBytes: []byte(msg),
	}
}

func (s *Session) InstallMessageGeneator(gen MessageGenerator) {
	s.genLock.Lock()
	defer s.genLock.Unlock()

	s.removeMessageGeneratorUnsafe()

	s.generator = gen
	s.generator.SetUid(s.ID)
	s.genClose = make(chan struct{})
	go func() {
		ticker := time.NewTicker(gen.Interval())
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.Sending <- gen.Generate()
			case <-s.genClose:
				return
			}
		}
	}()
}

func (s *Session) RemoveMessageGenerator() {
	s.genLock.Lock()
	defer s.genLock.Unlock()

	s.removeMessageGeneratorUnsafe()
}

func (s *Session) removeMessageGeneratorUnsafe() {
	if s.genClose != nil {
		close(s.genClose)
		s.genClose = nil
	}
}

func (s *Session) sendingWorker() {
	defer close(s.Sending)
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			continue
		case control := <-s.Control:
			switch control {
			case "close":
				s.Sending <- CloseMessage{}
				return
			default:
				log.Println("Received unhandled control message: ", control)
			}
		case msg := <-s.Sending:
			err := s.Conn.WriteMessage(msg.Type(), msg.Bytes())
			s.counter.Stat("sent", 1)
			if err != nil {
				log.Println("Error sending message: ", err)
				s.counter.Stat("sent:error", 1)
			}
		}
	}
}

func (s *Session) receivedWorker(id string) {
	defer s.Conn.Close()
	for {
		_, msg, err := s.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
				log.Println("Failed to read incoming message:", err)
				s.counter.Stat("received:error", 1)
				s.States <- "error"
			}
			s.counter.Stat("connected", -1)
			s.counter.Stat("disconnected", 1)
			s.States <- "closed"
			break
		}
		s.counter.Stat("received", 1)
		s.received <- MessageReceived{id, msg}
	}
}

func (s *Session) Close() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()
	s.Control <- "close"
}
