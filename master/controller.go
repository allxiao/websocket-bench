package master

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"net/rpc"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ArieShout/websocket-bench/agent"
	"github.com/ArieShout/websocket-bench/benchmark"
)

type AgentProxy struct {
	Address string
	Client  *rpc.Client
}

func NewAgentProxy(address string) (*AgentProxy, error) {
	client, err := rpc.DialHTTP("tcp", address)
	if err != nil {
		return nil, err
	}
	proxy := &AgentProxy{
		Address: address,
		Client:  client,
	}
	return proxy, nil
}

// Controller stands for a master and manages all the agents.
type Controller struct {
	Agents []*AgentProxy
}

func (c *Controller) RegisterAgent(address string) error {
	proxy, err := NewAgentProxy(address)
	if err != nil {
		return err
	}
	c.Agents = append(c.Agents, proxy)
	return nil
}

func (c *Controller) setupAgents(config *benchmark.Config) error {
	var wg sync.WaitGroup
	for _, agent := range c.Agents {
		wg.Add(1)
		go func(agent *AgentProxy) {
			if err := agent.Client.Call("Agent.Setup", config, &struct{}{}); err != nil {
				log.Fatalln(err)
			}
			wg.Done()
		}(agent)
	}

	wg.Wait()
	return nil
}

func (c *Controller) collectCounters() map[string]int64 {
	counters := make(map[string]int64)
	for _, agent := range c.Agents {
		result := make(map[string]int64)
		if err := agent.Client.Call("Agent.CollectCounters", &struct{}{}, &result); err != nil {
			log.Println("ERROR: Failed to list counters from agent: ", agent.Address, err)
		}
		for k, v := range result {
			counters[k] += v
		}
	}
	return counters
}

func (c *Controller) printCounters(counters map[string]int64) {
	table := make([][2]string, 0, len(counters))
	for k, v := range counters {
		table = append(table, [2]string{k, strconv.FormatInt(v, 10)})
	}

	sort.Slice(table, func(i, j int) bool {
		return table[i][0] < table[j][0]
	})

	log.Println("Counters:")
	for _, row := range table {
		log.Println("    ", row[0], ": ", row[1])
	}
}

func (c *Controller) SplitNumber(total, index int) int {
	agentCount := len(c.Agents)
	base := total / agentCount
	if index < total%agentCount {
		base++
	}
	return base
}

var csvHeader string
var counterFields = []string{
	"connection:inprogress",
	"connection:established",
	"connection:closed",
	"connection:error",
	"message:lt:100",
	"message:lt:200",
	"message:lt:300",
	"message:lt:400",
	"message:lt:500",
	"message:lt:600",
	"message:lt:700",
	"message:lt:800",
	"message:lt:900",
	"message:lt:1000",
	"message:gt:1000",
	"message:sent",
	"message:received",
	"message:send_error",
	"message:receive_error",
	"message:decode_error",
}

func formatCSVRecord(counters map[string]int64) string {
	values := make([]string, len(counterFields))
	for i, field := range counterFields {
		values[i] = strconv.FormatInt(counters[field], 10)
	}
	return strings.Join(values, ",")
}

func (c *Controller) Run(config *benchmark.Config) error {
	if err := c.setupAgents(config); err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)
	re := regexp.MustCompile("\\s+")
	for {
		fmt.Print("> ")
		text, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		parts := re.Split(text, -1)
		if len(parts) < 1 {
			continue
		}
		partsLen := len(parts)
		switch parts[0] {
		case "r":
			fallthrough
		case "result":
			c.printCounters(c.collectCounters())
		case "v":
			fmt.Println(csvHeader)
			counters := c.collectCounters()
			fmt.Println(formatCSVRecord(counters))
		case "c":
			fallthrough
		case "EnsureConnection":
			if partsLen < 2 || partsLen > 3 {
				fmt.Println("SYNTAX: c <connection_count> [connection_per_second]")
				break
			}
			connection, err := strconv.Atoi(parts[1])
			if err != nil {
				fmt.Println("ERROR: ", err)
				break
			}
			if connection < 0 {
				fmt.Println("ERROR: connection number is negative")
				break
			}
			connPerSecond := math.MaxInt32
			if len(parts) == 3 {
				connPerSecond, err = strconv.Atoi(parts[2])
				if err != nil {
					fmt.Println("ERROR: ", err)
					break
				}
			}
			if connPerSecond < 0 {
				fmt.Println("ERROR: connection per second is negative")
				break
			}

			for i, agentProxy := range c.Agents {
				agentConnection := c.SplitNumber(connection, i)
				agentConnPerSec := c.SplitNumber(connPerSecond, i)
				err := agentProxy.Client.Call("Agent.Invoke", &agent.Invocation{
					Command:   "EnsureConnection",
					Arguments: []string{strconv.Itoa(agentConnection), strconv.Itoa(agentConnPerSec)},
				}, nil)
				if err != nil {
					fmt.Printf("ERROR[%s]: %v\n", agentProxy.Address, err)
				}
			}
			connection -= connPerSecond
			if connection < 0 {
				break
			}
		case "s":
			fallthrough
		case "Send":
			if partsLen < 2 || partsLen > 3 {
				fmt.Println("SYNTAX: s <clients> [interval_millis]")
				break
			}
			clients, err := strconv.Atoi(parts[1])
			if err != nil {
				fmt.Println("ERROR: ", err)
				break
			}
			interval := 1000
			if partsLen >= 3 {
				interval, err = strconv.Atoi(parts[2])
				if err != nil {
					fmt.Println("ERROR: ", err)
					break
				}
			}
			if clients < 0 {
				clients = math.MaxInt32
			}
			for i, agentProxy := range c.Agents {
				agentClients := c.SplitNumber(clients, i)
				err := agentProxy.Client.Call("Agent.Invoke", &agent.Invocation{
					Command:   "Send",
					Arguments: []string{strconv.Itoa(agentClients), strconv.Itoa(interval)},
				}, nil)
				if err != nil {
					fmt.Printf("ERROR[%s]: %v\n", agentProxy.Address, err)
				}
			}
		default:
			for _, agentProxy := range c.Agents {
				err := agentProxy.Client.Call("Agent.Invoke", &agent.Invocation{
					Command:   parts[0],
					Arguments: parts[1:],
				}, nil)
				if err != nil {
					fmt.Printf("ERROR[%s]: %v\n", agentProxy.Address, err)
				}
			}
		}
	}

	return nil
}

func init() {
	headers := make([]string, len(counterFields))
	for i, field := range counterFields {
		parts := strings.SplitN(field, ":", 2)
		headers[i] = parts[1]
	}
	csvHeader = strings.Join(headers, ",")
}
