package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/NanoRed/lim/internal"
	"github.com/NanoRed/lim/pkg/logger"
	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/widgets/text"
	"github.com/mum4k/termdash/widgets/textinput"
)

var (
	ip   = flag.String("ip", "127.0.0.1", "input the server IP")
	port = flag.String("port", "7714", "input the server port")
)

func init() {
	if writer, err := os.Create("./limcli.log"); err != nil {
		logger.Panic("failed to register logger: %v", err)
	} else {
		logger.RegisterLogger(writer)
	}
}

func main() {
	flag.Parse()

	label := "sample"

	client := internal.NewClient(
		func() (net.Conn, error) {
			return net.Dial("tcp", fmt.Sprintf("%s:%s", *ip, *port))
		},
		internal.NewDefaultFrameProcessor(),
	)
	client.Connect()
	client.Label(label)

	// terminal
	terminal, err := tcell.New()
	if err != nil {
		logger.Panic("failed to new terminal: %v", err)
	}
	defer terminal.Close()

	ctx, cancel := context.WithCancel(context.Background())

	roll, err := text.New(text.RollContent(), text.WrapAtWords())
	if err != nil {
		logger.Panic("failed to create roll widget")
	}
	go func() {
		for {
			if l, msg := client.Receive(); l == label {
				err := roll.Write(fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04"), msg))
				if err != nil {
					logger.Panic("failed to write message into the roll widget")
				}
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
				client.Multicast(label, []byte(text))
			}
			return nil
		}),
		textinput.ClearOnSubmit(),
	)
	if err != nil {
		logger.Panic("failed to create input widget")
	}

	container, err := container.New(
		terminal,
		container.SplitHorizontal(
			container.Top(
				container.Border(linestyle.Light),
				container.BorderTitle(fmt.Sprintf("[Messages-%s]", label)),
				container.PlaceWidget(roll),
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
		logger.Panic("failed to create container")
	}

	if err := termdash.Run(ctx, terminal, container); err != nil {
		logger.Panic("failed to run terminal")
	}
}
