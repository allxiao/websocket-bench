package main

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"

	"github.com/ArieShout/websocket-bench/agent"
	flags "github.com/jessevdk/go-flags"
)

var opts struct {
	Mode          string `short:"m" long:"mode" description:"Run mode" default:"agent" choice:"agent" choice:"master"`
	OutputDir     string `short:"o" long:"output-dir" description:"Output directory" default:"output"`
	ListenAddress string `short:"l" long:"listen-address" description:"Listen address" default:":7000"`
	Agents        string `short:"a" long:"agents" description:"Agent addresses separated by comma"`
}

func startMaster() {
	client, err := rpc.DialHTTP("tcp", opts.Agents)
	if err != nil {
		log.Fatal("Failed to connect to the agent "+opts.Agents, err)
	}
	invocation := agent.Invocation{
		Command:   "Power",
		Arguments: []string{"2"},
	}
}

func startAgent() {
	rpc.RegisterName("AgentController", new(agent.RPC))
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", opts.ListenAddress)
	if err != nil {
		log.Fatal("Failed to listen on "+opts.ListenAddress, err)
	}
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
