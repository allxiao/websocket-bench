package benchmark

import (
	"time"

	"github.com/ArieShout/websocket-bench/util"
)

var _ Subject = (*SignalrServiceMsgpackBroadcast)(nil)

type SignalrServiceMsgpackBroadcast struct {
	SignalrServiceMsgpackEcho
}

func (s *SignalrServiceMsgpackBroadcast) InvokeTarget() string {
	return "broadcastMessage"
}

func (s *SignalrServiceMsgpackBroadcast) Name() string {
	return "SignalR Service MsgPack Broadcast"
}

func (s *SignalrServiceMsgpackBroadcast) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.ProcessMsgPackLatency(s.InvokeTarget())

	return nil
}

func (s *SignalrServiceMsgpackBroadcast) DoSend(clients int, intervalMillis int) error {
	return s.doSend(clients, intervalMillis, &MessagePackMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
		Target: s.InvokeTarget(),
	})
}
