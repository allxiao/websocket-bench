package master

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"net/rpc"
	"os"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	shellquote "github.com/kballard/go-shellquote"

	"github.com/ArieShout/websocket-bench/agent"
	"github.com/ArieShout/websocket-bench/benchmark"

	"github.com/bramvdbogaerde/go-scp/auth"
	"golang.org/x/crypto/ssh"
)

// AgentProxy is an RPC proxy to a remote agent.
type AgentProxy struct {
	Address string
	Client  *rpc.Client
}

// NewAgentProxy creates a new RPC proxy to a remote agent.
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

// RegisterAgent adds a new agent proxy to the master's agent set.
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

func (c *Controller) splitNumber(total, index int) int {
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
	"message:ge:1000",
	"message:sent",
	"message:received",
	"message:send_error",
	"message:receive_error",
	"message:decode_error",
}

var globalChannels []chan struct{}

func registerStopChannels(ch chan struct{}) {
	globalChannels = append(globalChannels, ch)
}

func (c *Controller) clearAllTask() {
	err := c.stopSending()
	if err != nil {
		fmt.Printf("Fail to stop sending %s\n", err)
	}
	err = c.closeConnection()
	if err != nil {
		fmt.Printf("Fail to close connections %s\n", err)
	}
	c.closeAllChan()
}

// Handle Ctrl C
func (c *Controller) handleSigterm() {
	fmt.Println("Handle Ctrl C")
	c.clearAllTask()
}

func (c *Controller) closeAllChan() {
	for _, ch := range globalChannels {
		close(ch)
	}
}

func (c *Controller) stopSending() error {
	var values []string
	values = append(values, "s")
	values = append(values, "0")
	fmt.Printf("Stop sending: %s\n", values)
	return c.send(values)
}

func (c *Controller) closeConnection() error {
	var values []string
	values = append(values, "c")
	values = append(values, "0")
	fmt.Printf("Close connections: %s\n", values)
	return c.connect(values)
}

func formatCSVRecord(counters map[string]int64) string {
	values := make([]string, len(counterFields))
	for i, field := range counterFields {
		values[i] = strconv.FormatInt(counters[field], 10)
	}
	return strings.Join(values, ",")
}

func (c *Controller) doInvoke(command string, arguments ...string) error {
	for _, agentProxy := range c.Agents {
		err := agentProxy.Client.Call("Agent.Invoke", &agent.Invocation{
			Command:   command,
			Arguments: arguments,
		}, nil)
		if err != nil {
			fmt.Printf("ERROR[%s] %s(%s): %v\n", agentProxy.Address, command, strings.Join(arguments, ","), err)
		}
	}

	return nil
}

func (c *Controller) watchCounters(config *benchmark.Config) {
	stopWatchCounterChan := make(chan struct{})
	registerStopChannels(stopWatchCounterChan)
	go c.watchCountersInternal(stopWatchCounterChan, func(counters map[string]int64) error {
		if config.OutDir != "" {
			SnapshotWriter := NewJsonSnapshotWriter(config.OutDir + "/counters.txt")
			if err := SnapshotWriter.WriteCounters(time.Now(), counters); err != nil {
				log.Println("Error: fail to write counter snapshot: ", err)
				return err
			}
		}
		return nil
	})
}

func (c *Controller) watchCountersInternal(stopChan chan struct{}, snapshotWriter func(map[string]int64) error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			counters := c.collectCounters()
			snapshotWriter(counters)
			c.printCounters(counters)
		case <-stopChan:
			return
		}
	}
}

func (c *Controller) waitTimeoutOrComplete(parts []string, stop bool) error {
	partsLen := len(parts)
	if partsLen < 2 || partsLen > 3 {
		return fmt.Errorf("SYNTAX: w <wait_time_seconds>")
	}
	timeoutSec, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("ERROR: %v", err)
	}
	if timeoutSec < 0 {
		return fmt.Errorf("ERROR: connection number is negative")
	}

	waitChannel := make(chan struct{})
	registerStopChannels(waitChannel)

	ticker := time.NewTicker(time.Second * time.Duration(timeoutSec))
	defer ticker.Stop()

	select {
	case <-ticker.C:
		fmt.Printf("--- Finished after %d sec ---\n", timeoutSec)
	case <-waitChannel:
		log.Println("--- Stopped ---")
	}
	if stop {
		c.clearAllTask()
	}
	return nil
}

const upperBound = 10000

func (c *Controller) autoRun(config *benchmark.Config) error {
	// TODO: parameterize control bounds and steps
	senderLimit := upperBound
	for i := 100; i <= 10000; i += 100 {
		fmt.Printf(">>> connect %d\n", i)
		c.connect([]string{"c", strconv.Itoa(i)})

		lower := 1
		upper := senderLimit
		if upper > i {
			upper = i
		}

		if !c.trySender(lower) {
			fmt.Printf("connection: %d, sender: 0", i)
			break
		}

		if c.trySender(upper) {
			fmt.Printf("connection: %d, sender: %d", i, upper)
			continue
		}

		for lower < upper {
			mid := lower + (upper-lower)/2
			if c.trySender(mid) {
				lower = mid + 1
			} else {
				upper = mid - 1
			}
		}

		fmt.Printf("connection: %d, sender: %d", i, lower)
	}
	return nil
}

const (
	ltPrefix    = "message:lt:"
	ltPrefixLen = len(ltPrefix)
	gtPrefix    = "message:gt:"
)

func (c *Controller) trySender(count int) bool {
	fmt.Printf(">>> try send %d\n", count)
	c.send([]string{"s", strconv.Itoa(count)})
	time.Sleep(5 * time.Second)

	pass := 0
	for i := 0; i < 5; i++ {
		c.doInvoke("Clear", "message")
		time.Sleep(10 * time.Second)
		counters := c.collectCounters()

		goodCount := 0
		total := 0
		for k, v := range counters {
			if strings.HasPrefix(k, ltPrefix) {
				latencyStr := k[ltPrefixLen:]
				latency, _ := strconv.Atoi(latencyStr)

				if latency <= 500 {
					goodCount += int(v)
				}
				total += int(v)
			} else if strings.HasPrefix(k, gtPrefix) {
				total += int(v)
			}
		}

		if total == 0 {
			fmt.Println("<<< Total is zero!")
			continue
		}

		ratio := float64(goodCount) / float64(total)

		fmt.Printf("<<< ratio: %.2f%%, %v\n", ratio*100, counters)

		if ratio >= 0.98 {
			pass++
		}

		time.Sleep(2 * time.Second)
	}

	return pass >= 3
}

func (c *Controller) batchRun(config *benchmark.Config) error {
	cmdFile := config.CmdFile
	file, err := os.Open(cmdFile)

	if err != nil {
		return fmt.Errorf("Fail to open %s", cmdFile)
	}
	defer func() {
		cerr := file.Close()
		if cerr != nil {
			fmt.Printf("Error occurs when close '%s'\n", cmdFile)
		}
	}()

	re := regexp.MustCompile("\\s+")
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		parts := re.Split(text, -1)
		if len(parts) < 1 {
			continue
		}
		switch parts[0] {
		case "r":
			fallthrough
		case "result":
			c.printCounters(c.collectCounters())
		case "cm":
			fallthrough
		case "ClearMessage":
			c.doInvoke("Clear", "message")
		case "wr":
			fallthrough
		case "WatchResult":
			c.watchCounters(config)
		case "c":
			fallthrough
		case "EnsureConnection":
			err = c.connect(parts)
			if err != nil {
				fmt.Println(err)
				return err
			}
		case "s":
			fallthrough
		case "Send":
			err = c.send(parts)
			if err != nil {
				fmt.Println(err)
				return err
			}
		case "wc":
			fallthrough
		case "WaitAndContinue":
			err = c.waitTimeoutOrComplete(parts, false)
			if err != nil {
				fmt.Println(err)
				return err
			}
		case "w":
			fallthrough
		case "Wait":
			err = c.waitTimeoutOrComplete(parts, true)
			if err != nil {
				fmt.Println(err)
				return err
			}
		default:
			fmt.Printf("Illegal command!")
			return fmt.Errorf("illegal command")
		}
	}
	return nil
}

func (c *Controller) interactiveRun() error {
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
		switch parts[0] {
		case "r":
			fallthrough
		case "result":
			c.printCounters(c.collectCounters())
		case "v":
			c.clearAndWaitAndDump(10)
		case "c":
			fallthrough
		case "EnsureConnection":
			err = c.connect(parts)
			if err != nil {
				fmt.Println(err)
				break
			}
		case "s":
			fallthrough
		case "Send":
			err = c.send(parts)
			if err != nil {
				fmt.Println(err)
				break
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
}

func (c *Controller) clearAndWaitAndDump(secWait int) {
	c.doInvoke("Clear", "message")
	time.Sleep(time.Duration(secWait) * time.Second)
	fmt.Println(csvHeader)
	counters := c.collectCounters()
	fmt.Println(formatCSVRecord(counters))
}

func (c *Controller) connect(parts []string) error {
	partsLen := len(parts)
	if partsLen < 2 || partsLen > 3 {
		return fmt.Errorf("SYNTAX: c <connection_count> [connection_per_second]")
	}
	connection, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("ERROR: %v", err)
	}
	if connection < 0 {
		return fmt.Errorf("ERROR: connection number is negative")
	}
	connPerSecond := math.MaxInt32
	if len(parts) == 3 {
		connPerSecond, err = strconv.Atoi(parts[2])
		if err != nil {
			return fmt.Errorf("ERROR: %v", err)
		}
	}
	if connPerSecond < 0 {
		return fmt.Errorf("ERROR: connection per second is negative")
	}

	for i, agentProxy := range c.Agents {
		agentConnection := c.splitNumber(connection, i)
		agentConnPerSec := c.splitNumber(connPerSecond, i)
		err := agentProxy.Client.Call("Agent.Invoke", &agent.Invocation{
			Command:   "EnsureConnection",
			Arguments: []string{strconv.Itoa(agentConnection), strconv.Itoa(agentConnPerSec)},
		}, nil)
		if err != nil {
			return fmt.Errorf("ERROR[%s]: %v", agentProxy.Address, err)
		}
	}
	return nil
}

func (c *Controller) send(parts []string) error {
	partsLen := len(parts)
	if partsLen < 2 || partsLen > 3 {
		return fmt.Errorf("SYNTAX: s <clients> [interval_millis]")
	}
	clients, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("ERROR: %v", err)
	}
	interval := 1000
	if partsLen >= 3 {
		interval, err = strconv.Atoi(parts[2])
		if err != nil {
			return fmt.Errorf("ERROR: %v", err)
		}
	}
	if clients < 0 {
		clients = math.MaxInt32
	}
	for i, agentProxy := range c.Agents {
		agentClients := c.splitNumber(clients, i)
		err := agentProxy.Client.Call("Agent.Invoke", &agent.Invocation{
			Command:   "Send",
			Arguments: []string{strconv.Itoa(agentClients), strconv.Itoa(interval)},
		}, nil)
		if err != nil {
			return fmt.Errorf("ERROR[%s]: %v", agentProxy.Address, err)
		}
	}
	return nil
}

func (c *Controller) createOutDir(outDir string) {
	if outDir != "" {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			log.Fatalln(err)
		}
		log.Println("Ouptut directory: ", outDir)
	}
}

// Run starts the master node.
func (c *Controller) Run(config *benchmark.Config) error {
	if err := c.setupAgents(config); err != nil {
		return err
	}

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		c.handleSigterm()
		os.Exit(1)
	}()

	c.createOutDir(config.OutDir)
	if config.CmdFile == "" {
		return c.interactiveRun()
	}
	return c.batchRun(config)
}

func init() {
	headers := make([]string, len(counterFields))
	for i, field := range counterFields {
		parts := strings.SplitN(field, ":", 2)
		headers[i] = parts[1]
	}
	csvHeader = strings.Join(headers, ",")
}

// StartAgents copies the current running executable to remote hosts and starts the process without arguments.
func (c *Controller) StartAgents(hosts []string) {
	fmt.Printf("starting %d agents: %v\n", len(hosts), hosts)
	user, err := user.Current()
	if err != nil {
		log.Fatalln(err)
	}
	keyPath := path.Join(user.HomeDir, ".ssh", "id_rsa")
	clientConfig, err := auth.PrivateKey(user.Username, keyPath, ssh.InsecureIgnoreHostKey())
	if err != nil {
		log.Fatalln(err)
	}

	executablePath, err := filepath.Abs(os.Args[0])
	if err != nil {
		log.Fatalln(err)
	}

	executable := filepath.Base(executablePath)

	for _, host := range hosts {
		fmt.Printf("dial %s@%s\n", user.Username, host)
		sshClient, err := ssh.Dial("tcp", host, &clientConfig)
		if err != nil {
			log.Fatalln("Cannot connect to SSH "+host, err)
		}
		defer sshClient.Close()

		session, err := sshClient.NewSession()
		if err != nil {
			log.Fatalln("Cannot new SSH session to "+host, err)
		}
		defer session.Close()

		session.Run(shellquote.Join("killall", executable))

		scpClient := scp.NewClient(host, &clientConfig)

		if err := scpClient.Connect(); err != nil {
			log.Fatalln("Cannot connect SSH "+host+" for scp", err)
		}

		f, err := os.Open(executablePath)
		if err != nil {
			log.Fatalln("Cannot open executable "+executablePath, err)
		}
		defer scpClient.Session.Close()
		defer f.Close()

		scpClient.CopyFile(f, executable, "0755")

		session, err = sshClient.NewSession()
		if err != nil {
			log.Fatalln("Cannot new session for executable on "+host, err)
		}

		if err := session.Run("ulimit -n 200000; " + shellquote.Join("nohup", "./"+executable) + " >output.log 2>&1 &"); err != nil {
			log.Fatalln("Cannot start agent on "+host, err)
		}
		fmt.Println("Started agent on", host)
	}

	fmt.Println("Wait for 10 seconds for the agents start up...")
	time.Sleep(time.Second * 10)
}
