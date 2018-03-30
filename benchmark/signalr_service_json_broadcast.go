package benchmark

import (
	"time"

	"aspnet.com/util"
)

var _ Subject = (*SignalrServiceJsonBroadcast)(nil)

type SignalrServiceJsonBroadcast struct {
	SignalrServiceJsonEcho
}

func (s *SignalrServiceJsonBroadcast) InvokeTarget() string {
	return "broadcastMessage"
}

func (s *SignalrServiceJsonBroadcast) Name() string {
	return "SignalR Service Broadcast"
}

func (s *SignalrServiceJsonBroadcast) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.ProcessJsonLatency(s.InvokeTarget())

	return nil
}

func (s *SignalrServiceJsonBroadcast) DoSend(clients int, intervalMillis int) error {
	return s.doSend(clients, intervalMillis, &SignalRCoreTextMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
		Target: s.InvokeTarget(),
	})
}
