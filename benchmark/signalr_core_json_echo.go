package benchmark

import (
	"time"

	"github.com/ArieShout/websocket-bench/util"
)

var _ Subject = (*SignalrCoreJsonEcho)(nil)

type SignalrCoreJsonEcho struct {
	SignalrCoreCommon
}

func (s *SignalrCoreJsonEcho) InvokeTarget() string {
	return "echo"
}

func (s *SignalrCoreJsonEcho) Name() string {
	return "SignalR Core Connection"
}

func (s *SignalrCoreJsonEcho) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.ProcessJsonLatency(s.InvokeTarget())

	return nil
}

func (s *SignalrCoreJsonEcho) DoEnsureConnection(count int, conPerSec int) error {
	return s.doEnsureConnection(count, conPerSec, func(withSessions *WithSessions) (*Session, error) {
		return s.SignalrCoreJsonConnect()
	})
}

func (s *SignalrCoreJsonEcho) DoSend(clients int, intervalMillis int) error {
	s.counter.Clear("connection:stop_sender")
	s.counter.Clear("connection:sender_stoped")
	return s.doSend(clients, intervalMillis, &SignalRCoreTextMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
		Target: s.InvokeTarget(),
	})
}
