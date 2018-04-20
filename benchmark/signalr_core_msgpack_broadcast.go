package benchmark

import (
	"time"

	"github.com/ArieShout/websocket-bench/util"
)

var _ Subject = (*SignalrCoreMsgpackBroadcast)(nil)

type SignalrCoreMsgpackBroadcast struct {
	SignalrCoreMsgpackEcho
}

func (s *SignalrCoreMsgpackBroadcast) InvokeTarget() string {
	return "broadcastMessage"
}

func (s *SignalrCoreMsgpackBroadcast) Name() string {
	return "SignalR Core MessagePack"
}

func (s *SignalrCoreMsgpackBroadcast) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.ProcessMsgPackLatency(s.InvokeTarget())

	return nil
}

func (s *SignalrCoreMsgpackBroadcast) DoSend(clients int, intervalMillis int) error {
	return s.doSend(clients, intervalMillis, &MessagePackMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
		Target: s.InvokeTarget(),
	})
}
