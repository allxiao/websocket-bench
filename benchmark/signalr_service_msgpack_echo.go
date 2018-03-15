package benchmark

import (
	"time"

	"aspnet.com/util"
)

var _ Subject = (*SignalrServiceMsgpackEcho)(nil)

func (s *SignalrServiceMsgpackEcho) InvokeTarget() string {
	return "echo"
}

type SignalrServiceMsgpackEcho struct {
	SignalrCoreCommon
}

func (s *SignalrServiceMsgpackEcho) Name() string {
	return "SignalR Service MsgPack Echo"
}

func (s *SignalrServiceMsgpackEcho) Setup(config *Config) error {
	s.host = config.Host

	s.counter = util.NewCounter()
	s.sessions = make([]*Session, 0, 20000)

	s.received = make(chan MessageReceived)

	go s.ProcessMsgPackLatency(s.InvokeTarget())

	return nil
}

func (s *SignalrServiceMsgpackEcho) DoEnsureConnection(count int, conPerSec int) error {
	return s.doEnsureConnection(count, conPerSec, func(withSessions *WithSessions) (*Session, error) {
		return s.SignalrServiceMsgPackConnect()
	})
}

func (s *SignalrServiceMsgpackEcho) DoSend(clients int, intervalMillis int) error {
	return s.doSend(clients, intervalMillis, &MessagePackMessageGenerator{
		WithInterval: WithInterval{
			interval: time.Millisecond * time.Duration(intervalMillis),
		},
		Target: s.InvokeTarget(),
	})
}
