package chat

import (
	"api"
	"google.golang.org/grpc/metadata"
	Timestamp "google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"log"
	"sync"
)

type ChatServer struct {
	api.UnimplementedChatServer
	mu      sync.Mutex
	streams []*api.Chat_RouteMessagesServer
}

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

		if in.Status == api.Status_Online {
			s.mu.Lock()

			var found *api.Chat_RouteMessagesServer
			for _, existed := range s.streams {
				if existed == &stream {
					found = existed
					break
				}
			}
			if found == nil {
				s.streams = append(s.streams, &stream)
			}
			s.mu.Unlock()

			log.Printf("(%+v): registered\n", from)
		} else if in.Status == api.Status_Offline {
			s.mu.Lock()

			for i, existed := range s.streams {
				if existed == &stream {
					s.streams = append(s.streams[:i], s.streams[i+1:]...)
					break
				}
			}
			s.mu.Unlock()

			log.Printf("(%+v): unregistered\n", from)
		} else {
			log.Printf("(%+v): \"%+v\"\n", from, *in.Text)
		}

		message := &api.Message{Timestamp: Timestamp.Now(), From: &from, Text: in.Text, Status: in.Status}

		// sending message to all registered streams
		s.mu.Lock()
		offlineMessages := make([]*api.Message, 0)
		for i := 0; i < len(s.streams); i++ {
			clientStream := s.streams[i]
			if err := (*clientStream).Send(message); err != nil {

				streamUsername := metadata.ValueFromIncomingContext((*clientStream).Context(), "username")[0]
				log.Printf("(%+v): offline\n", streamUsername)

				offlineMessages = append(offlineMessages, &api.Message{Timestamp: Timestamp.Now(), From: &streamUsername, Status: api.Status_Offline})

				// remove and process next client stream
				s.streams = append(s.streams[:i], s.streams[i+1:]...)
				i--
				break
			}
		}
		s.mu.Unlock()

		// sending offline status to all registered streams
		if len(offlineMessages) > 0 {
			for _, offlineMessage := range offlineMessages {
				for _, clientStream := range s.streams {
					if err := (*clientStream).Send(offlineMessage); err != nil {
						streamUsername := metadata.ValueFromIncomingContext((*clientStream).Context(), "username")[0]
						log.Printf("(%+v): offline\n", streamUsername)
					}
				}
			}
		}
	}
}
