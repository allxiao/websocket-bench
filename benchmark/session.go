package benchmark

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ArieShout/websocket-bench/util"
	"github.com/gorilla/websocket"
)

const (
	SessionInitialized       = 0
	SessionConnecting        = iota
	SessionWaitingHandshake  = iota
	SessionHandshakeReceived = iota
	SessionEstablished       = iota
	SessionClosing           = iota
	SessionClosed            = iota
	SessionTerminated        = iota
)

// Session represents a single connection to the given websocket host.
type Session struct {
	ID       string
	Conn     *websocket.Conn
	Control  chan string
	Sending  chan Message
	received chan MessageReceived
	States   chan string
	state    int32

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
	atomic.StoreInt32(&s.state, SessionInitialized)
	s.genLock = sync.Mutex{}
	return s
}

func (s *Session) Start() {
	go s.sendingWorker()
	go s.receivedWorker(s.ID)
}

func (s *Session) NegotiateProtocol(protocol string) {
	s.WriteTextMessage("{\"protocol\":\"" + protocol + "\",\"version\":1}\x1e")
	atomic.StoreInt32(&s.state, SessionWaitingHandshake)
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

	if atomic.LoadInt32(&s.state) >= SessionClosing {
		return
	}

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
				s.Sending <- gen.Generate(s.ID, 0)
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
	fmt.Println("sending message: ", string(msg.Bytes()))
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
				if atomic.LoadInt32(&s.state) < SessionClosing {
					s.counter.Stat("connection:closing", 1)
					s.sendMessage(CloseMessage{})
				}
				return
			case "terminated":
				atomic.StoreInt32(&s.state, SessionTerminated)
				// closed abnormally, no need to issue close request from the client side
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
				s.counter.Stat("connection:terminated", 1)
				s.RemoveMessageGenerator()
				s.Control <- "terminated"
				s.States <- "terminated"
			} else {
				if atomic.LoadInt32(&s.state) == SessionClosing {
					s.counter.Stat("connection:closing", -1)
					s.counter.Stat("connection:closed", 1)
				} else {
					s.counter.Stat("connection:closed_remotely", 1)
				}
				s.States <- "closed"
			}
			break
		}
		s.counter.Stat("message:received", 1)
		s.counter.Stat("message:recvSize", int64(len(msg)))
		if atomic.LoadInt32(&s.state) == SessionWaitingHandshake {
			dataArray := bytes.Split(msg, []byte{0x1e})
			if len(dataArray[0]) == 2 {
				// empty json "{}"
				atomic.StoreInt32(&s.state, SessionHandshakeReceived)

				s.counter.Stat("connection:inprogress", -1)
				s.counter.Stat("connection:established", 1)
			} else {
				fmt.Printf("handshake fail because %s\n", dataArray[0])
			}
		} else {
			s.received <- MessageReceived{id, msg}
		}
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
