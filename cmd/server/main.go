package main

import (
	"api"
	"chat"
	"google.golang.org/grpc"
	"log"
	"net"
)

func main() {
	s := grpc.NewServer()
	srv := &chat.ChatServer{}
	api.RegisterChatServer(s, srv)

	l, err := net.Listen("tcp", ":5000")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Chat server started at localhost:5000")

	if err := s.Serve(l); err != nil {
		log.Fatal(err)
	}
}
