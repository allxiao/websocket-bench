package benchmark

// Config defines the basic configuration for the benchmark.
type Config struct {
	Host          string
	Subject       string
	CmdFile       string
	OutDir        string
	Mode          string
	AutorunConfig AutorunConfig
}

type AutorunConfig struct {
	StartConnection int
	EndConnection   int
	Step            int
	Round           int
	PassRound       int
	QosRatio        float64
}

func NewAutorunConfig(process func(*AutorunConfig)) AutorunConfig {
	cfg := AutorunConfig{
		StartConnection: 100,
		EndConnection:   10000,
		Step:            500,
		Round:           5,
		PassRound:       0,
		QosRatio:        0.98,
	}
	if process != nil {
		configured := AutorunConfig{}
		process(&configured)

		if configured.StartConnection > 0 {
			cfg.StartConnection = configured.StartConnection
		}

		if configured.EndConnection > 0 {
			cfg.EndConnection = configured.EndConnection
		}

		if configured.Step > 0 {
			cfg.Step = configured.Step
		}

		if configured.Round > 0 {
			cfg.Round = configured.Round
		}

		if configured.PassRound > 0 {
			cfg.PassRound = configured.PassRound
		}

		if configured.QosRatio > 0 {
			cfg.QosRatio = configured.QosRatio
		}
	}

	if cfg.PassRound <= 0 {
		cfg.PassRound = cfg.Round
	}

	return cfg
}
