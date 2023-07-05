package main

import (
	"flag"
	"fmt"

	"github.com/NanoRed/lim/internal"
)

var (
	ip       = flag.String("ip", "127.0.0.1", "input the server IP")
	port     = flag.String("port", "7714", "input the server port")
	wssPort  = flag.String("wssPort", "7715", "input the SSL websocket server port")
	certFile = flag.String("cert", "/etc/letsencrypt/live/wizard.red/fullchain.pem", "input the SSL certificate path")
	keyFile  = flag.String("key", "/etc/letsencrypt/live/wizard.red/privkey.pem", "input the SSL key path")
)

func main() {
	flag.Parse()

	server := internal.NewServer()
	server.EnableWSS(fmt.Sprintf("%s:%s", *ip, *wssPort), *certFile, *keyFile)
	server.EnableWebsite(fmt.Sprintf("%s:443", *ip), *certFile, *keyFile)
	server.ListenAndServe(fmt.Sprintf("%s:%s", *ip, *port))
}
