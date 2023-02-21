package main

import (
	"flag"

	"github.com/NanoRed/lim/pkg/logger"
	"github.com/NanoRed/lim/pkg/server"
)

var ip = flag.String("ip", "127.0.0.1", "input the IP you want to listen to")
var port = flag.String("port", "7714", "input the port you want to listion to")

func main() {
	flag.Parse()
	addr := *ip + ":" + *port
	logger.Info("you are listening %s", addr)
	s := server.NewServer(addr)
	if err := s.ListenAndServe(); err != nil {
		logger.Fatal("failed to serve: %v", err)
	}
}
