package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"syscall/js"
	"time"

	"github.com/NanoRed/lim/internal"
	"github.com/NanoRed/lim/pkg/logger"
)

var (
	addr string = "ws://127.0.0.1:7715/"
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	name := pokemonNames[r.Intn(55)]
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
			orderMutex := make(chan struct{}, 1)
			orderMutex <- struct{}{}
			fileReader := js.Global().Get("FileReader").New()
			fileReader.Set("onload", js.FuncOf(func(this js.Value, args []js.Value) any {
				arrayBuffer := fileReader.Get("result")
				jsArray := js.Global().Get("Uint8Array").New(arrayBuffer)
				bytes := make([]byte, jsArray.Length())
				js.CopyBytesToGo(bytes, jsArray)
				conn2.Write(bytes)
				orderMutex <- struct{}{}
				return nil
			}))
			ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
				<-orderMutex
				fileReader.Call("readAsArrayBuffer", args[0].Get("data"))
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
				logger.Info("successfully connected to the websocket server")
				ok <- struct{}{}
				return nil
			}))
			ws.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) any {
				logger.Info("disconnected from websocket server")
				conn1.Close()
				conn2.Close()
				return nil
			}))
			js.Global().Set("onbeforeunload", js.FuncOf(func(this js.Value, args []js.Value) any {
				once.Do(func() {
					ws.Call("close")
					conn1.Close()
					conn2.Close()
				})
				return nil
			}))
			<-ok
			close(ok)
			return conn1, nil
		},
		internal.NewDefaultFrameProcessor(),
	)
	wg := &sync.WaitGroup{}
	connected := make(chan struct{}, 1)
	// func: lim_websocket_connect
	js.Global().Set("lim_websocket_connect", js.FuncOf(func(this js.Value, args []js.Value) any {
		wg.Add(1)
		go func() {
			if err := client.Connect(); err != nil {
				logger.Error("connect failed: %v", err)
			} else {
				select {
				case connected <- struct{}{}:
				default:
				}
			}
			wg.Done()
		}()
		return nil
	}))
	// func: lim_websocket_label
	js.Global().Set("lim_websocket_label", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			go func() {
				wg.Wait()
				if err := client.Label(args[0].String()); err != nil {
					logger.Error("label failed: %v", err)
				}
			}()
			return nil
		}
		return js.Global().Get("Error").New("invalid arguments")
	}))
	// func: lim_websocket_dislabel
	js.Global().Set("lim_websocket_dislabel", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			go func() {
				wg.Wait()
				if err := client.Dislabel(args[0].String()); err != nil {
					logger.Error("dislabel failed: %v", err)
				}
			}()
			return nil
		}
		return js.Global().Get("Error").New("invalid arguments")
	}))
	// func: lim_websocket_multicast
	js.Global().Set("lim_websocket_multicast", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 1 {
			go func() {
				wg.Wait()
				if err := client.Multicast(
					args[0].String(),
					[]byte(fmt.Sprintf("[%s]%s: %s", time.Now().Format("15:04:05"), name, args[1].String())),
				); err != nil {
					logger.Error("multicast failed: %v", err)
				}
			}()
			return nil
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

var pokemonNames = [55]string{
	"Pikachu",
	"Bulbasaur",
	"Charmander",
	"Squirtle",
	"Jigglypuff",
	"Meowth",
	"Psyduck",
	"Growlithe",
	"Poliwag",
	"Abra",
	"Machop",
	"Tentacool",
	"Geodude",
	"Magnemite",
	"Grimer",
	"Shellder",
	"Gastly",
	"Onix",
	"Drowzee",
	"Krabby",
	"Voltorb",
	"Exeggcute",
	"Cubone",
	"Hitmonlee",
	"Hitmonchan",
	"Lickitung",
	"Koffing",
	"Rhyhorn",
	"Chansey",
	"Tangela",
	"Kangaskhan",
	"Horsea",
	"Goldeen",
	"Staryu",
	"Scyther",
	"Jynx",
	"Electabuzz",
	"Magmar",
	"Pinsir",
	"Tauros",
	"Magikarp",
	"Lapras",
	"Ditto",
	"Eevee",
	"Porygon",
	"Omanyte",
	"Kabuto",
	"Aerodactyl",
	"Snorlax",
	"Articuno",
	"Zapdos",
	"Moltres",
	"Dratini",
	"Dragonair",
	"Dragonite",
}
