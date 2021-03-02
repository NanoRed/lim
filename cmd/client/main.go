package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/RedAFD/lim/pkg/client"
	"github.com/RedAFD/lim/pkg/handler"
	"github.com/RedAFD/lim/pkg/logger"
	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/widgets/text"
	"github.com/mum4k/termdash/widgets/textinput"
)

var ip = flag.String("ip", "127.0.0.1", "input the IP you want to dial")
var port = flag.String("port", "7714", "input the port you want to dial")

func main() {
	flag.Parse()

	// dial
	addr := *ip + ":" + *port
	c := client.NewClient(addr)
	h, err := c.DialForHandler(nil)
	if err != nil {
		logger.Fatal("failed to dial %v", err)
	}
	logger.Info("successful dial %s", addr)
	handler := h.(*handler.DefaultCliHandler)
	handler.Label("global")

	// terminal
	terminal, err := tcell.New()
	if err != nil {
		panic(err)
	}
	defer terminal.Close()

	ctx, cancel := context.WithCancel(context.Background())

	rolled, err := text.New(text.RollContent(), text.WrapAtWords())
	if err != nil {
		panic(err)
	}
	go func() {
		var msg string
		for {
			if label, message, err := handler.ConsumeTasks(); err == nil {
				msg = fmt.Sprintf("[%s] <%s> %s\n", time.Now().Format("15:04"), label, message)
			} else {
				msg = fmt.Sprintf("[%s] <sys> an error occurred: %v\n", time.Now().Format("15:04"), err)
			}
			if err := rolled.Write(msg); err != nil {
				panic(err)
			}
		}
	}()

	input, err := textinput.New(
		textinput.FillColor(cell.ColorNumber(0)),
		textinput.WidthPerc(100),
		textinput.OnSubmit(func(text string) error {
			if text == ":q" {
				cancel()
			} else {
				handler.Broadcast("global", []byte(text))
			}
			return nil
		}),
		textinput.ClearOnSubmit(),
	)
	if err != nil {
		panic(err)
	}

	container, err := container.New(
		terminal,
		container.SplitHorizontal(
			container.Top(
				container.Border(linestyle.Light),
				container.BorderTitle("[Message]"),
				container.PlaceWidget(rolled),
			),
			container.Bottom(
				container.Border(linestyle.Light),
				container.BorderTitle("[Enter]"),
				container.PlaceWidget(input),
			),
			container.SplitPercent(85),
		),
	)
	if err != nil {
		panic(err)
	}

	if err := termdash.Run(ctx, terminal, container); err != nil {
		panic(err)
	}
}
