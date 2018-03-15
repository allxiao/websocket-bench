package benchmark

import (
	"time"

	"aspnet.com/util"
)

var _ Subject = (*SignalrServiceJsonEcho)(nil)

type SignalrServiceJsonEcho struct {
	SignalrCoreCommon
}

func (s *SignalrServiceJsonEcho) InvokeTarget() string {
	return "echo"
}

func (s *SignalrServiceJsonEcho) Name() string {
	return "SignalR Service Echo"
}

func (s *SignalrServiceJsonEcho) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.ProcessJsonLatency(s.InvokeTarget())

	return nil
}


func (s *SignalrServiceJsonEcho) DoEnsureConnection(count int, conPerSec int) error {
	return s.doEnsureConnection(count, conPerSec, func(withSessions *WithSessions) (*Session, error) {
		return s.SignalrServiceJsonConnect()
	})
}

func (s *SignalrServiceJsonEcho) DoSend(clients int, intervalMillis int) error {
	return s.doSend(clients, intervalMillis, &SignalRCoreTextMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
		Target: s.InvokeTarget(),
	})
}
