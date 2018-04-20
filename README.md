# websocket-bench

Inspired from https://github.com/ArchangelSDY/Sigbench .

## Build

```bash
go build -tags forceposix github.com/ArieShout/websocket-bench
```

## Run

* Agent

   ```bash
   websocket-bench
   ```

* Master interactive mode

   ```bash
   websocket-bench -m master -a <agent-hosts> -s <websocket-server> -t signalr:json:echo
   ```

   (You can find the supported topics from `SubjectMap` in [agent/controller.go](agent/controller.go))

   The master starts a REPL environment where you can send commands interactively:

   * `c <connection> [connection_per_second]`

      Ensure we have the target number of connections to the server. This can be used to increase the connection
      number if the current number of established connections are lower than the target number, or decrease the
      connections if greater.

   * `s <senders> [interval]`

      Set the number of the senders which will send a message to the server every `[interval]` (default `1000`) milliseconds.
      Run `s 0` to stop sending messages.

   * `r`

      Instantly get the current benchmark statistics data in raw format.

   * `v`

      Get the benchmark statistics of the next 10 seconds in CSV format. It's performed in the following way:

      1. Clear the current statistics data (with interactive command `Clear message`)
      1. Wait for 10 seconds
      1. Get the statistics data and format it in CSV

* Master batch command mode

   Batch mode is to support running this benchmark in a script. All the commands you want to run are written to a file.
   Every line only has one command. Some commands are specially for this mode:

   * `wr`
      
      "Watch Result" (wr) dumps the latency numbers as well as connection statistic on master node.

   * `w <second>`

      "Wait" command tells the master node to stop the test after the specified duration. It is useful if
      you want to stop your test gracefully after a fixed time duration.

   * `wc <second>`

      "Wait and Countine <second>" is an extension of "wait" command. It does not stop the test after the 
      specified duration.

   * `cm`

      "Clear Message" wants to clean all the history latency statistics. It does not remove the connections statistic.


   ```bash
   ./websocket-bench -m master -a "localhost:7000" -s "172.17.8.4:5050" -t signalr:json:echo -c json-echo-cmds.txt -o signalr_json_echo
   ```
   Here is the command list of json-echo-cmds.txt:

```txt
   c 16000
   s 15000
   wr
   wc 60
   cm
   w 360
```
      Send message to 15000 clients after 16000 connections were established. Watch the result on the master node.
      Clean all latency statistic after 60 seconds, then collect new statistics for 360 seconds and stop.
