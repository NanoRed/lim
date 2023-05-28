package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/NanoRed/lim/pkg/logger"
	"github.com/NanoRed/lim/website"
)

var (
	ip   = flag.String("ip", "127.0.0.1", "input the server IP")
	port = flag.String("port", "80", "input the server port")
)

func main() {
	flag.Parse()

	http.Handle("/", http.FileServer(http.FS(website.FS)))
	if err := http.ListenAndServe(fmt.Sprintf("%s:%s", *ip, *port), nil); err != nil {
		logger.Error("website server error: %v", err)
	}
}
