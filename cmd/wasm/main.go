package main

import (
	"bufio"
	"fmt"
	"net"
	"syscall/js"

	"github.com/NanoRed/lim/internal"
	"github.com/NanoRed/lim/internal/websocket"
	"github.com/NanoRed/lim/pkg/logger"
)

var (
	ip   string = "127.0.0.1"
	port string = "7715"
)

func init() {
	in, out := net.Pipe()
	go func() {
		reader := bufio.NewReader(out)
		for {
			line, _, _ := reader.ReadLine()
			js.Global().Get("console").Call("log", string(line))
		}
	}()
	logger.RegisterLogger(in)
}

func main() {
	client := internal.NewClient(
		websocket.NewDialer(ip, port),
		internal.NewDefaultFrameProcessor(),
	)
	// func: lim_websocket_connect
	js.Global().Set("lim_websocket_connect", js.FuncOf(func(this js.Value, args []js.Value) any {
		if err := client.Connect(); err != nil {
			return js.Global().Get("Error").New(fmt.Sprintf("%v", err))
		}
		return nil
	}))
	// func: lim_websocket_label
	js.Global().Set("lim_websocket_label", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			if err := client.Label(args[0].String()); err == nil {
				return nil
			} else {
				return js.Global().Get("Error").New(fmt.Sprintf("%v", err))
			}
		}
		return js.Global().Get("Error").New("invalid arguments")
	}))
	// func: lim_websocket_dislabel
	js.Global().Set("lim_websocket_dislabel", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			if err := client.Dislabel(args[0].String()); err == nil {
				return nil
			} else {
				return js.Global().Get("Error").New(fmt.Sprintf("%v", err))
			}
		}
		return js.Global().Get("Error").New("invalid arguments")
	}))
	// func: lim_websocket_multicast
	js.Global().Set("lim_websocket_multicast", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 1 {
			if err := client.Multicast(args[0].String(), []byte(args[1].String())); err == nil {
				return nil
			} else {
				return js.Global().Get("Error").New(fmt.Sprintf("%v", err))
			}
		}
		return js.Global().Get("Error").New("invalid arguments")
	}))
	// invoke: lim_websocket_onreceive
	go func() {
		for {
			label, message := client.Receive()
			if fn := js.Global().Get("lim_websocket_onreceive"); fn.Type() == js.TypeFunction {
				fn.Invoke(label, string(message))
			}
		}
	}()
	select {}
}
