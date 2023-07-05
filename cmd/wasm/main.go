package main

import (
	"bufio"
	"net"
	"sync"
	"syscall/js"
	"time"

	"github.com/NanoRed/lim/internal"
	"github.com/NanoRed/lim/pkg/logger"
)

var (
	addr string = "wss://127.0.0.1:7715/"
)

func init() {
	internal.ResponseTimeout = time.Second * 10
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
		func() (net.Conn, error) {
			once := &sync.Once{}
			ok := make(chan struct{})
			conn1, conn2 := net.Pipe()
			ws := js.Global().Get("WebSocket").New(addr)
			go func() {
				b := make([]byte, 4096)
				for {
					if n, err := conn2.Read(b); err != nil {
						once.Do(func() {
							ws.Call("close")
							conn1.Close()
							conn2.Close()
						})
						logger.Error("pipe closed: %v", err)
						return
					} else {
						jsArray := js.Global().Get("Uint8Array").New(n)
						js.CopyBytesToJS(jsArray, b[:n])
						ws.Call("send", jsArray)
					}
				}
			}()
			sequencer := make(chan struct{}, 1)
			sequencer <- struct{}{}
			ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
				<-sequencer
				conn2.Write(jsBlobToGoBytes(args[0].Get("data")))
				sequencer <- struct{}{}
				return nil
			}))
			ws.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
				once.Do(func() {
					ws.Call("close")
					conn1.Close()
					conn2.Close()
				})
				logger.Error("error occured: %s", args[0].Get("message").String())
				return nil
			}))
			ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) any {
				ok <- struct{}{}
				logger.Info("successfully connected to the websocket server")
				return nil
			}))
			ws.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) any {
				conn1.Close()
				conn2.Close()
				logger.Info(
					"disconnected from websocket server (code:%s reason:%s)",
					jsStringToGoBytes(js.Global().Get("String").Invoke(args[0].Get("code"))),
					jsStringToGoBytes(args[0].Get("reason")),
				)
				return nil
			}))
			js.Global().Set("onbeforeunload", js.FuncOf(func(this js.Value, args []js.Value) any {
				once.Do(func() {
					ws.Call("close")
					conn1.Close()
					conn2.Close()
				})
				if fn := js.Global().Get("lim_websocket_onunload"); fn.Type() == js.TypeFunction {
					fn.Invoke()
				}
				return nil
			}))
			<-ok
			close(ok)
			return conn1, nil
		},
	)
	wg := &sync.WaitGroup{}
	connected := make(chan struct{}, 1)
	// func: lim_websocket_connect
	js.Global().Set("lim_websocket_connect", js.FuncOf(func(this js.Value, args []js.Value) any {
		wg.Add(1)
		if err := client.Connect(); err != nil {
			logger.Error("connect failed: %v", err)
		} else {
			select {
			case connected <- struct{}{}:
			default:
			}
		}
		wg.Done()
		return nil
	}))
	// func: lim_websocket_label
	js.Global().Set("lim_websocket_label", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Type() == js.TypeString {
			wg.Wait()
			label := string(jsStringToGoBytes(args[0]))
			if err := client.Label(label); err != nil {
				logger.Error("label failed: %v", err)
			} else {
				logger.Info("label successfully: %s", label)
			}
			return nil
		}
		return js.Global().Get("Error").New("invalid arguments")
	}))
	// func: lim_websocket_dislabel
	js.Global().Set("lim_websocket_dislabel", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Type() == js.TypeString {
			wg.Wait()
			label := string(jsStringToGoBytes(args[0]))
			if err := client.Dislabel(label); err != nil {
				logger.Error("dislabel failed: %v", err)
			} else {
				logger.Info("label successfully: %s", label)
			}
			return nil
		}
		return js.Global().Get("Error").New("invalid arguments")
	}))
	// func: lim_websocket_multicast
	type streamTool struct {
		stream chan []byte
		state  int
		mu     *sync.Mutex
	}
	newStreamTool := &sync.Pool{New: func() any {
		return &streamTool{
			stream: make(chan []byte),
			state:  0,
			mu:     &sync.Mutex{},
		}
	}}
	onceMap := &sync.Map{}
	js.Global().Set("lim_websocket_multicast", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 1 && args[0].Type() == js.TypeString {
			label := string(jsStringToGoBytes(args[0]))
			switch args[1].Type() {
			case js.TypeString:
				wg.Wait()
				if err := client.Multicast(label, jsStringToGoBytes(args[1])); err != nil {
					logger.Error("multicast failed: %v", err)
				}
				return nil
			case js.TypeObject:
				wg.Wait()
				nst := newStreamTool.Get().(*streamTool)
				st, ok := onceMap.LoadOrStore(label, nst)
				streamTool := st.(*streamTool)
				if ok {
					newStreamTool.Put(nst)
				} else {
					go func() {
						if err := client.Multicast(label, streamTool.stream); err != nil {
							logger.Error("multicast failed: %v", err)
						}
					}()
				}
				streamTool.mu.Lock()
				if streamTool.state != 0 {
					streamTool.mu.Unlock()
					return nil
				}
				element := args[1].Call("shift")
				if element.Type() == js.TypeUndefined {
					streamTool.state = -1
					close(streamTool.stream)
					onceMap.Delete(label)
					streamTool.mu.Unlock()
					return nil
				}
				payload := make([]byte, element.Length())
				js.CopyBytesToGo(payload, element)
				streamTool.stream <- payload
				streamTool.mu.Unlock()
				return nil
			}
		}
		return js.Global().Get("Error").New("invalid arguments")
	}))
	// invoke: lim_websocket_onreceive
	go func() {
		for {
			label, messages := client.Receive()
			if fn := js.Global().Get("lim_websocket_onreceive"); fn.Type() == js.TypeFunction {
				for _, message := range messages {
					jsArray := js.Global().Get("Uint8Array").New(len(message))
					js.CopyBytesToJS(jsArray, message)
					fn.Invoke(js.ValueOf(label), jsArray)
				}
			}
		}
	}()
	// invoke: lim_websocket_onload
	go func() {
		for {
			<-connected
			if fn := js.Global().Get("lim_websocket_onload"); fn.Type() == js.TypeFunction {
				fn.Invoke()
			}
		}
	}()
	select {}
}

var jsStringToGoBytes = func() func(val js.Value) []byte {
	textEncoder := js.Global().Get("TextEncoder").New()
	return func(val js.Value) []byte {
		eocodedText := textEncoder.Call("encode", val)
		textArray := js.Global().Get("Uint8Array").New(eocodedText)
		textBytes := make([]byte, textArray.Length())
		js.CopyBytesToGo(textBytes, textArray)
		return textBytes
	}
}()

var jsBlobToGoBytes = func() func(val js.Value) []byte {
	result := make(chan []byte)
	sequencer := make(chan struct{}, 1)
	sequencer <- struct{}{}
	fileReader := js.Global().Get("FileReader").New()
	fileReader.Set("onload", js.FuncOf(func(this js.Value, args []js.Value) any {
		arrayBuffer := args[0].Get("target").Get("result")
		jsArray := js.Global().Get("Uint8Array").New(arrayBuffer)
		bytes := make([]byte, jsArray.Length())
		js.CopyBytesToGo(bytes, jsArray)
		result <- bytes
		sequencer <- struct{}{}
		return nil
	}))
	return func(val js.Value) []byte {
		<-sequencer
		fileReader.Call("readAsArrayBuffer", val)
		return <-result
	}
}()
