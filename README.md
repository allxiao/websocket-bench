# websocket-bench

Inspired from https://github.com/ArchangelSDY/Sigbench .

## Build

```sh
go build -tags forceposix github.com/ArieShout/websocket-bench
```

## Run

* Agent

   ```sh
   websocket-bench
   ```

* Master

   ```sh
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
