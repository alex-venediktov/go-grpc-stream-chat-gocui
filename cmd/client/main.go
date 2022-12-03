package main

import (
	"api"
	"bufio"
	"context"
	"fmt"
	"github.com/jroimartin/gocui"
	"github.com/tjarratt/babble"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	Timestamp "google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

func runRouteMessage(ctx context.Context, client api.ChatClient, input <-chan *string, g *gocui.Gui) {

	SinceTimestamp := Timestamp.Now()

	babbler := babble.NewBabbler()
	babbler.Separator = " "

	stream, err := client.RouteMessages(ctx)
	if err != nil {
		log.Fatalf("client.RouteMessages failed: %v", err)
	}

	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				time.Sleep(250 * time.Millisecond)
				continue
			}
			if err != nil {
				log.Fatalf("client.RouteMessages failed: %v", err)
			}

			if SinceTimestamp == nil || in.Timestamp.AsTime().After(SinceTimestamp.AsTime()) {
				SinceTimestamp = in.Timestamp
			}

			g.Update(func(g *gocui.Gui) error {
				v, err := g.View("list")
				if err != nil {
					return err
				}
				fmt.Fprintln(v, in.Timestamp.AsTime().Format(time.RFC3339)+" ("+in.From+"): "+in.Text)
				return nil
			})
		}
	}()

	go func() {
		for {
			time.Sleep(250 * time.Millisecond)
			message := api.MessageRequest{
				Text:           nil,
				SinceTimestamp: SinceTimestamp,
			}
			if err := stream.Send(&message); err != nil {
				log.Fatalf("client.RouteMessage: stream.Send(%v) failed: %v", message, err)
			}
		}
	}()

	go func() {
		for {
			time.Sleep(time.Duration(rand.Intn(120)) * time.Second)

			babbler.Count = rand.Intn(3) + 1
			text := babbler.Babble()

			message := api.MessageRequest{
				Text:           &text,
				SinceTimestamp: SinceTimestamp,
			}
			if err := stream.Send(&message); err != nil {
				log.Fatalf("client.RouteMessage: stream.Send(%v) failed: %v", message, err)
			}
		}
	}()

	go func() {
		for {
			select {
			case text := <-input:
				if *text != "" {
					message := api.MessageRequest{
						Text:           text,
						SinceTimestamp: SinceTimestamp,
					}
					if err := stream.Send(&message); err != nil {
						log.Fatalf("client.RouteMessage: stream.Send(%v) failed: %v", message, err)
					}
				}
			}
		}
	}()

	<-waitc
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	username := ""
	for {
		fmt.Print("Enter your name: ")
		username, _ = reader.ReadString('\n')
		username = strings.Trim(username, " \r\n")
		if username != "" {
			break
		}
	}

	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("username", username),
	)

	conn, err := grpc.Dial(":5000", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	chat := api.NewChatClient(conn)

	// guid init
	input := make(chan *string)

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.Cursor = true
	g.Mouse = true

	g.SetManagerFunc(layout)

	if err := g.SetKeybinding("main", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}

	if err := g.SetKeybinding("main", gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		text := v.Buffer()
		text = strings.Trim(text, " \r\n")
		v.Clear()
		v.SetCursor(0, 0)
		input <- &text
		return nil
	}); err != nil {
		log.Panicln(err)
	}

	// run threads
	go runRouteMessage(ctx, chat, input, g)

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if v, err := g.SetView("main", 0, 0, maxX-1, 5); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Enter message"
		v.Editable = true
		v.Wrap = true
		if _, err := g.SetCurrentView("main"); err != nil {
			return err
		}
	}
	if v, err := g.SetView("list", 0, 6, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Chat"
		v.Wrap = true
		v.Autoscroll = true
	}
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
