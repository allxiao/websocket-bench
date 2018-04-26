package benchmark

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ArieShout/websocket-bench/util"
)

// Subject defines the interface for a test subject.
type Subject interface {
	Name() string
	Setup(config *Config) error
	Counters() map[string]int64

	DoEnsureConnection(count int, conPerSec int) error
	DoSend(clients int, intervalMillis int) error
	DoClear(prefix string) error
}

type WithCounter struct {
	counter *util.Counter
}

func (w *WithCounter) Counter() *util.Counter {
	return w.counter
}

func (w *WithCounter) LogError(errorGroup string, uid string, msg string, err error) {
	log.Printf("[Error][%s] %s due to %s", uid, msg, err)
	if errorGroup != "" {
		w.Counter().Stat(errorGroup, 1)
	}
}

func (w *WithCounter) LogLatency(latency int64) {
	index := int(latency / 100)
	if index >= 10 {
		w.Counter().Stat("message:ge:1000", 1)
	} else {
		w.Counter().Stat(fmt.Sprintf("message:lt:%d00", index+1), 1)
	}
}

func (s *WithCounter) Counters() map[string]int64 {
	return s.Counter().Snapshot()
}

func (s *WithCounter) DoClear(prefix string) error {
	s.counter.Clear(prefix)
	return nil
}

type WithSessions struct {
	host string

	sessions     []*Session
	sessionsLock sync.Mutex

	received chan MessageReceived
}

func (s *WithSessions) doEnsureConnection(count int, conPerSec int, builder func(*WithSessions) (*Session, error)) error {
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
				session, err := builder(s)
				if err != nil {
					return err
				}
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

func (s *WithSessions) doSend(clients int, intervalMillis int, gen MessageGenerator) error {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	s.doStopSendUnsafe()

	sessionCount := len(s.sessions)
	bound := sessionCount
	if clients < bound {
		bound = clients
	}

	indices := rand.Perm(sessionCount)
	for i := 0; i < bound; i++ {
		s.sessions[indices[i]].InstallMessageGeneator(gen)
	}

	return nil
}

func (s *WithSessions) DoStopSend() error {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	return s.doStopSendUnsafe()
}

func (s *WithSessions) doStopSendUnsafe() error {
	for _, session := range s.sessions {
		session.RemoveMessageGenerator()
	}

	return nil
}
