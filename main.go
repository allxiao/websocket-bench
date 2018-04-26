package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strings"

	"github.com/ArieShout/websocket-bench/agent"
	"github.com/ArieShout/websocket-bench/benchmark"
	"github.com/ArieShout/websocket-bench/master"
	flags "github.com/jessevdk/go-flags"
)

var opts struct {
	Mode          string `short:"m" long:"mode" description:"Run mode" default:"agent" choice:"agent" choice:"master" choice:"interactive"`
	OutputDir     string `short:"o" long:"output-dir" description:"Output directory" default:"output"`
	ListenAddress string `short:"l" long:"listen-address" description:"Listen address" default:":7000"`
	Agents        string `short:"a" long:"agents" description:"Agent addresses separated by comma"`
	Server        string `short:"s" long:"server" description:"Websocket server host:port"`
	Subject       string `short:"t" long:"test-subject" description:"Test subject"`
	CmdFile       string `short:"c" long:"cmd-file" description:"Command file"`

	StartConnection int     `long:"start-connection" description:"[Autorun] The connection count to start with."`
	EndConnection   int     `long:"end-connection" description:"[Autorun] The upper bound of connection count to stop at."`
	Step            int     `long:"step" description:"[Autorun] How many connection should be increased in each iteration."`
	Round           int     `long:"round" description:"[Autorun] How many rounds to check in each iteration."`
	PassRound       int     `long:"pass-round" description:"[Autorun] How many rounds is required to pass the QoS ratio when we mark the iteration as pass."`
	QosRatio        float64 `long:"qos-ratio" description:"[Autorun] What is the target ratio for the messages whose RTT is less than 500ms."`
}

func startMaster() {
	agentHosts := strings.Split(opts.Agents, ",")
	if len(agentHosts) <= 0 {
		log.Fatalln("No agents specified")
	}
	log.Println("Agents: ", agentHosts)

	agentSSHHosts := make([]string, 0, len(agentHosts))
	agentEndpoints := make([]string, 0, len(agentHosts))
	for _, host := range agentHosts {
		host = strings.TrimSpace(host)
		agentSSHHosts = append(agentSSHHosts, host+":22")
		agentEndpoints = append(agentEndpoints, host+":7000")
	}

	c := &master.Controller{}

	c.StartAgents(agentSSHHosts)

	doStartInteractiveMaster(c, agentEndpoints)
}

func startInteractive() {
	agentAddresses := strings.Split(opts.Agents, ",")
	if len(agentAddresses) <= 0 {
		log.Fatalln("No agents specified")
	}
	log.Println("Agents: ", agentAddresses)

	c := &master.Controller{}

	doStartInteractiveMaster(c, agentAddresses)
}

func doStartInteractiveMaster(c *master.Controller, agents []string) {
	if opts.Server == "" {
		log.Fatalln("Server host:port was not specified")
	}

	if opts.Subject == "" {
		log.Fatalln("Subject was not specified")
	}

	genPidFile("/tmp/websocket-bench-master.pid")

	for _, address := range agents {
		if err := c.RegisterAgent(address); err != nil {
			log.Fatalln("Failed to register agent: ", address, err)
		}
	}

	c.Run(&benchmark.Config{
		Host:    opts.Server,
		Subject: opts.Subject,
		CmdFile: opts.CmdFile,
		OutDir:  opts.OutputDir,
		Mode:    opts.Mode,
		AutorunConfig: benchmark.NewAutorunConfig(func(cfg *benchmark.AutorunConfig) {
			cfg.StartConnection = opts.StartConnection
			cfg.EndConnection = opts.EndConnection
			cfg.Step = opts.Step
			cfg.Round = opts.Round
			cfg.PassRound = opts.PassRound
			cfg.QosRatio = opts.QosRatio
		}),
	})
}

func genPidFile(pidfile string) {
	f, _ := os.Create(pidfile)
	defer func() {
		cerr := f.Close()
		if cerr != nil {
			log.Fatalln("Failed to close the pid file: ", cerr)
		}
	}()
	_, err := f.WriteString(fmt.Sprintf("%d", os.Getpid()))
	if err != nil {
		log.Println("Fail to write pidfile")
	}
}
func startAgent() {
	rpc.RegisterName("Agent", new(agent.Controller))
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", opts.ListenAddress)
	if err != nil {
		log.Fatal("Failed to listen on "+opts.ListenAddress, err)
	}
	log.Println("Listen on ", l.Addr())
	genPidFile("/tmp/websocket-bench.pid")
	http.Serve(l, nil)
}

func main() {
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	if opts.Mode == "interactive" {
		startInteractive()
	} else if opts.Mode == "master" {
		startMaster()
	} else {
		startAgent()
	}
}
