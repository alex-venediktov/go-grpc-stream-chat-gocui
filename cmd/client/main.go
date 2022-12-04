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
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func Colored(text string, i, j int) string {
	return "\033[3" + strconv.Itoa(i) + ";" + strconv.Itoa(j) + "m" + text + "\033[0m"
}

func runRouteMessage(ctx context.Context, client api.ChatClient, input <-chan *string, g *gocui.Gui) {

	babbler := babble.NewBabbler()
	babbler.Separator = " "

	stream, err := client.RouteMessages(ctx)
	if err != nil {
		log.Fatalf("client.RouteMessages failed: %v", err)
	}

	// execute once to register client stream on server
	message := api.Message{
		From:   &Username,
		Status: api.Status_Online,
	}
	if err := stream.Send(&message); err != nil {
		log.Fatalf("client.RouteMessage: stream.Send(%v) failed: %v", message, err)
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

			g.Update(func(g *gocui.Gui) error {
				v, err := g.View("list")
				if err != nil {
					return err
				}

				message := ""
				switch in.Status {
				case api.Status_Text:
					message = Colored(*in.Text, 7, 1)
				case api.Status_AutoText:
					message = Colored(*in.Text, 0, 1)
				case api.Status_Online:
					message = Colored("[online now]", 7, 7)
				case api.Status_Offline:
					message = Colored("[offline now]", 7, 7)
				}

				fmt.Fprintln(v, in.Timestamp.AsTime().Format(time.RFC3339)+" ("+*in.From+"): "+message)
				return nil
			})
		}
	}()

	go func() {
		for {
			time.Sleep(time.Duration(rand.Intn(120)) * time.Second)

			babbler.Count = rand.Intn(3) + 1
			text := babbler.Babble()

			message := api.Message{
				Text:   &text,
				Status: api.Status_AutoText,
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
					message := api.Message{
						Text:   text,
						Status: api.Status_Text,
					}
					if err := stream.Send(&message); err != nil {
						log.Fatalf("client.RouteMessage: stream.Send(%v) failed: %v", message, err)
					}
				}
			}
		}
	}()

	<-waitc

	// execute to unregister client stream on server
	message = api.Message{
		From:   &Username,
		Status: api.Status_Offline,
	}
	if err := stream.Send(&message); err != nil {
		log.Fatalf("client.RouteMessage: stream.Send(%v) failed: %v", message, err)
	}
}

var (
	Username string
)

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	username := ""
	for {
		fmt.Print("Enter your name (ASCII symbols only): ")
		username, _ = reader.ReadString('\n')
		username = strings.Trim(username, " \r\n")
		if username != "" && isASCII(username) {
			break
		}
	}

	Username = username

	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("username", Username),
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
		v.Title = "Enter message, " + Username + ":"
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
