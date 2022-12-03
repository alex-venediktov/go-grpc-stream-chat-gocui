package chat

import (
	"api"
	"google.golang.org/grpc/metadata"
	Timestamp "google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"log"
	"sync"
	"time"
)

type ChatServer struct {
	api.UnimplementedChatServer
	mu sync.Mutex
}

var Messages = NewRepository[api.Message](func(obj any) any {
	return obj.(*api.Message).Timestamp
})

func (s *ChatServer) RouteMessages(stream api.Chat_RouteMessagesServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		from := metadata.ValueFromIncomingContext(stream.Context(), "username")[0]

		if in.Text != nil {
			message := api.Message{
				Timestamp: Timestamp.Now(),
				From:      from,
				Text:      *in.Text,
			}
			s.mu.Lock()
			Messages.Add(&message)
			s.mu.Unlock()

			log.Printf("%+v (%+v): \"%+v\"\n", time.Now().Format(time.RFC3339), from, *in.Text)
		} else {
			//log.Printf("%+v (%+v): \"%+v\"\n", time.Now().Format(time.RFC3339), from, nil)
		}

		s.mu.Lock()
		messages := Messages.Filter(func(m *api.Message) bool {
			return in.SinceTimestamp == nil || in.SinceTimestamp.AsTime().Before(m.Timestamp.AsTime())
		})
		s.mu.Unlock()

		for _, message := range messages {
			if err := stream.Send(message); err != nil {
				return err
			}
		}
	}
}
