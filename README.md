# websocket-bench

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
