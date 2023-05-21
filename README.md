# lim
a pure golang lightful IM system
### Usage
For more information, please review the cmd directory
```golang
// server
package main

import "github.com/NanoRed/lim/internal"

func main() {
    server := internal.NewServer("127.0.0.1:7714", internal.NewDefaultFrameProcessor())
	server.ListenAndServe()
}
```
```golang
// client
package main

import "github.com/NanoRed/lim/pkg/client"

func main() {
    client := internal.NewClient("127.0.0.1:7714", internal.NewDefaultFrameProcessor())
	client.Connect()

	client.Label("global") // label this connection on the server
    // client.Dislabel("test")

    go func() {
        // open a goroutine to consume messages from the service side
        for {
            label, message := client.Receive()
            logger.Info("%s %s", label, message)
		}
    }()
    client.
    handler.Multicast("global", []byte("hello world"))

    for{
    }
}
```
### Development Trends
- ☑️ basic available tcp server
- ☑️ connections manager(based on label)
- ☑️ protocol and custom protocol interface
- ☑️ logger and custom logger interface
- ☑️ complex and robust client implement
- ☑️ client connection heartbeat
- ☑️ simple authentication
- ☑️ backoff delay reconnection
- 🟦 label memory
- 🟦 better authentication
- 🟦 websocket support
- 🟦 cluster support
- 🟦 docs
