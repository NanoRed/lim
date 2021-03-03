# lim
a pure golang lightful IM system
### Usage
For more information, please review the cmd directory
```golang
// server
package main

import "github.com/RedAFD/lim/pkg/server"

func main() {
    s := server.NewServer("127.0.0.1:7714")
	if err := s.ListenAndServe(); err != nil {
        logger.Fatal("failed to serve: %v", err)
    }
}
```
```golang
// client
package main

import "github.com/RedAFD/lim/pkg/client"

func main() {
    c := client.NewClient("127.0.0.1:7714")
    h, err := c.DialForHandler(nil)
    if err != nil {
        logger.Fatal("failed to dial %v", err)
    }

    // actions
    handler := h.(*handler.DefaultCliHandler)
    handler.Label("global")
    handler.Dislabel("team")

    go func() {
        for {
            // open a goroutine to consume task queues from the service side
            if label, message, err := handler.ConsumeTasks(); err == nil {
                logger.Info("%s %s %v", label, message, err)
            }
        }
    }()
    time.Sleep(time.Second)
    handler.Broadcast("global", []byte("hello world"))

    for{
    }
}
```
### Development Trends
- ☑️ basic available tcp server
- ☑️ connections label manager
- ☑️ simple customizable protocol(handler)
- ☑️ simple customizable logger
- ☑️ client
- ☑️ heartbeat
- ☑️ cmd main application
- 🟦 Optimize the protocol package volume
- 🟦 websocket support
- 🟦 authentication
- 🟦 cluster support
- 🟦 events interceptor
- 🟦 docs
