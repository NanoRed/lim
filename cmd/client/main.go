package main

import (
	"context"
	"flag"
	"fmt"
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
	ip    = flag.String("ip", "106.52.81.44", "input the IP you want to dial")
	port  = flag.String("port", "7715", "input the port you want to dial")
	label = "sample"
)

func main() {
	flag.Parse()

	client := internal.NewClient(*ip+":"+*port, internal.NewDefaultFrameProcessor())
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
