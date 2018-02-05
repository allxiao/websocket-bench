package benchmark

import (
	"time"

	"github.com/ArieShout/websocket-bench/util"
)

var _ Subject = (*SignalrServiceBroadcast)(nil)

type SignalrServiceBroadcast struct {
	SignalrServiceEcho
}

func (s *SignalrServiceBroadcast) Name() string {
	return "SignalR Service Broadcast"
}

func (s *SignalrServiceBroadcast) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.processLatency("broadcastMessage")

	return nil
}

func (s *SignalrServiceBroadcast) DoSend(clients int, intervalMillis int) error {
	return s.doSend(clients, intervalMillis, &SignalRCoreTextMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
		Target: "broadcastMessage",
	})
}
