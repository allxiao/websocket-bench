package benchmark

import (
	"time"

	"github.com/ArieShout/websocket-bench/util"
)

var _ Subject = (*SignalrCoreJsonBroadcast)(nil)

type SignalrCoreJsonBroadcast struct {
	SignalrCoreJsonEcho
}

func (s *SignalrCoreJsonBroadcast) InvokeTarget() string {
	return "broadcastMessage"
}

func (s *SignalrCoreJsonBroadcast) Name() string {
	return "SignalR Core Connection"
}

func (s *SignalrCoreJsonBroadcast) Setup(config *Config) error {
	s.host = config.Host
	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)
	s.received = make(chan MessageReceived)
	go s.ProcessJsonLatency(s.InvokeTarget())

	return nil
}

func (s *SignalrCoreJsonBroadcast) DoSend(clients int, intervalMillis int) error {
	return s.doSend(clients, intervalMillis, &SignalRCoreTextMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
		Target: s.InvokeTarget(),
	})
}
