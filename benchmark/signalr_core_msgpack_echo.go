package benchmark

import (
	"time"

	"github.com/ArieShout/websocket-bench/util"
)

var _ Subject = (*SignalrCoreMsgpackEcho)(nil)

type SignalrCoreMsgpackEcho struct {
	SignalrCoreCommon
}

func (s *SignalrCoreMsgpackEcho) InvokeTarget() string {
	return "echo"
}

func (s *SignalrCoreMsgpackEcho) Name() string {
	return "SignalR Core MessagePack"
}

func (s *SignalrCoreMsgpackEcho) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.ProcessMsgPackLatency(s.InvokeTarget())

	return nil
}

func (s *SignalrCoreMsgpackEcho) DoEnsureConnection(count int, conPerSec int) error {
	return s.doEnsureConnection(count, conPerSec, func(withSessions *WithSessions) (*Session, error) {
		return s.SignalrCoreMsgPackConnect()
	})
}

func (s *SignalrCoreMsgpackEcho) DoSend(clients int, intervalMillis int) error {
	return s.doSend(clients, intervalMillis, &MessagePackMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
		Target: s.InvokeTarget(),
	})
}
