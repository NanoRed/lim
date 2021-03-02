# lim
a pure golang lightful IM system
### Usage
```golang
// server
package main

import "github.com/RedAFD/lim/pkg/server"

func main() {
    s := server.NewServer("127.0.0.1:7714")
	s.ListenAndServe()
}

// client
```
### Development Trends
- ☑️ basic available tcp server
- ☑️ connections label manager
- ☑️ simple customizable protocol(handler)
- ☑️ simple customizable logger
- ☑️ client
- ☑️ heartbeat
- ☑️ cmd main application
- 🟦 websocket support
- 🟦 authentication
- 🟦 cluster support
- 🟦 events interceptor
- 🟦 docs