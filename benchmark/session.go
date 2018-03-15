package benchmark

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"aspnet.com/util"
	"github.com/gorilla/websocket"
)

// Session represents a single connection to the given websocket host.
type Session struct {
	ID       string
	Conn     *websocket.Conn
	Control  chan string
	Sending  chan Message
	received chan MessageReceived
	States   chan string

	invocationId int64

	counter *util.Counter

	genLock  sync.Mutex
	genClose chan struct{}
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

	s.genClose = make(chan struct{})
	go func() {
		// randomize the start time of the generator
		time.Sleep(time.Millisecond * time.Duration(rand.Int()%1000))
		ticker := time.NewTicker(gen.Interval())
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.Sending <- gen.Generate(s.ID, s.invocationId)
				s.invocationId++
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

func (s *Session) sendMessage(msg Message) {
	err := s.Conn.WriteMessage(msg.Type(), msg.Bytes())
	s.counter.Stat("message:sent", 1)
	s.counter.Stat("message:sendSize", int64(len(msg.Bytes())))
	if err != nil {
		log.Println("Error sending message: ", err)
		s.counter.Stat("message:send_error", 1)
	}
}

func (s *Session) sendingWorker() {
	for {
		select {
		case control := <-s.Control:
			switch control {
			case "close":
				s.counter.Stat("connection:closing", 1)
				s.sendMessage(CloseMessage{})
				return
			default:
				log.Println("Received unhandled control message: ", control)
			}
		case msg := <-s.Sending:
			s.sendMessage(msg)
		}
	}
}

func (s *Session) receivedWorker(id string) {
	defer s.Conn.Close()
	for {
		_, msg, err := s.Conn.ReadMessage()
		if err != nil {
			s.counter.Stat("connection:established", -1)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
				log.Println("Failed to read incoming message:", err)
				s.counter.Stat("message:receive_error", 1)
				s.States <- "error"
			} else {
				s.counter.Stat("connection:closing", -1)
				s.counter.Stat("connection:closed", 1)
				s.States <- "closed"
			}
			break
		}
		s.counter.Stat("message:received", 1)
		s.counter.Stat("message:recvSize", int64(len(msg)))
		s.received <- MessageReceived{id, msg}
	}
}

func (s *Session) Close() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()
	s.RemoveMessageGenerator()
	s.Control <- "close"
}
