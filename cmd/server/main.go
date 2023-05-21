package main

import (
	"flag"

	"github.com/NanoRed/lim/internal"
	"github.com/NanoRed/lim/pkg/logger"
)

var (
	ip   = flag.String("ip", "127.0.0.1", "input the IP you want to listen to")
	port = flag.String("port", "7714", "input the port you want to listion to")
)

func main() {
	flag.Parse()

	server := internal.NewServer(*ip+":"+*port, internal.NewDefaultFrameProcessor())
	logger.Pure("Lim server started")
	server.ListenAndServe()
	logger.Pure("server exited")
}
