# lim
a pure golang lightful IM system
### Usage
For more information, please review the cmd directory
```golang
// server
package main

import "github.com/NanoRed/lim/internal"

func main() {
    server := internal.NewServer(internal.NewDefaultFrameProcessor())
	server.EnableWebsocket("127.0.0.1:7715")
	server.ListenAndServe("127.0.0.1:7714")
}
```
```golang
// client
package main

import "github.com/NanoRed/lim/pkg/client"

func main() {
    client := internal.NewClient(
		func() (net.Conn, error) { return net.Dial("tcp", "127.0.0.1:7714") },
		internal.NewDefaultFrameProcessor(),
	)
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
    client.Multicast("global", []byte("hello world"))

    select{}
}
```
```html
<!-- websocket client -->
<head>
    <script src="wasm_exec.js"></script>
    <script>
        const go = new Go();
        WebAssembly.instantiateStreaming(fetch("limcli.wasm"), go.importObject)
            .then((result) => {
                go.run(result.instance);

                // *exposed function: lim_websocket_connect
                // connect to the websocket server
                lim_websocket_connect(); 

                // *exposed function: lim_websocket_label
                // label this connection on the server
                lim_websocket_label("global");
            })

        // *exposed function: lim_websocket_onload
        // when websocket successfully connected, 
        // the runtime will invoke this function
        function lim_websocket_onload() {
            // do something
        }

        function sendMessage(message) {
            // *exposed function: lim_websocket_multicast
            // multicast a message to specific label group
            lim_websocket_multicast("global", message);
        }

        // *invoke function: lim_websocket_onreceive
        // after you define a function with this name,
        // when a message arrives, the runtime will invoke
        // this function
        function lim_websocket_onreceive(label, message) {
            console.log(label, message);
        }
    </script>
</head>
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
- ☑️ relabel when reconnecting
- ☑️ websocket support
- 🟦 better authentication
- 🟦 cluster support
- 🟦 docs
