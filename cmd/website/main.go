package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/NanoRed/lim/pkg/logger"
	"github.com/NanoRed/lim/website"
)

var (
	ip       = flag.String("ip", "127.0.0.1", "input the server IP")
	port     = flag.String("port", "443", "input the server port")
	certFile = flag.String("cert", "/etc/letsencrypt/live/wizard.red/fullchain.pem", "input the SSL certificate path")
	keyFile  = flag.String("key", "/etc/letsencrypt/live/wizard.red/privkey.pem", "input the SSL key path")
)

func main() {
	flag.Parse()

	http.Handle("/", http.FileServer(http.FS(website.ChatRoomFS)))
	if err := http.ListenAndServeTLS(fmt.Sprintf("%s:%s", *ip, *port), *certFile, *keyFile, nil); err != nil {
		logger.Error("website server error: %v", err)
	}
}
