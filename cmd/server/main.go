package main

import (
	"flag"
	"fmt"

	"github.com/NanoRed/lim/internal"
)

var (
	ip     = flag.String("ip", "127.0.0.1", "input the server IP")
	port   = flag.String("port", "7714", "input the server port")
	wsPort = flag.String("wsPort", "7715", "input the websocket server port")
)

func main() {
	flag.Parse()

	server := internal.NewServer()
	server.EnableWebsocket(fmt.Sprintf("%s:%s", *ip, *wsPort))
	server.ListenAndServe(fmt.Sprintf("%s:%s", *ip, *port))
}
