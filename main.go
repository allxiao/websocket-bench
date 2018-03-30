package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strings"

	"aspnet.com/agent"
	"aspnet.com/benchmark"
	"aspnet.com/master"
	flags "github.com/jessevdk/go-flags"
)

var opts struct {
	Mode          string `short:"m" long:"mode" description:"Run mode" default:"agent" choice:"agent" choice:"master"`
	OutputDir     string `short:"o" long:"output-dir" description:"Output directory" default:"output"`
	ListenAddress string `short:"l" long:"listen-address" description:"Listen address" default:":7000"`
	Agents        string `short:"a" long:"agents" description:"Agent addresses separated by comma"`
	Server        string `short:"s" long:"server" description:"Websocket server host:port"`
	Subject       string `short:"t" long:"test-subject" description:"Test subject"`
	CmdFile       string `short:"c" long:"cmd-file" description:"Command file"`
}

func startMaster() {
	agentAddresses := strings.Split(opts.Agents, ",")
	if len(agentAddresses) <= 0 {
		log.Fatalln("No agents specified")
	}
	log.Println("Agents: ", agentAddresses)

	if opts.Server == "" {
		log.Fatalln("Server host:port was not specified")
	}

	if opts.Subject == "" {
		log.Fatalln("Subject was not specified")
	}

	genPidFile("/tmp/websocket-bench-master.pid")

	c := &master.Controller{}

	for _, address := range agentAddresses {
		if err := c.RegisterAgent(address); err != nil {
			log.Fatalln("Failed to register agent: ", address, err)
		}
	}

	c.Run(&benchmark.Config{
		Host:    opts.Server,
		Subject: opts.Subject,
		CmdFile: opts.CmdFile,
		OutDir:  opts.OutputDir,
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

	if opts.Mode == "master" {
		startMaster()
	} else {
		startAgent()
	}
}
