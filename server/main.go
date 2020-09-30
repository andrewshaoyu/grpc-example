package main

import (
	"fmt"
	pb "go-test/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
	"log"
	"net"
	"time"
)

type server struct {
	userMap map[string]*Connection
	bigMap  map[string]chan *pb.Msg
}

type Connection struct {
	Peer        string
	PushChannel chan *pb.Msg
	Stream      pb.ChatService_ChatServer
}

func (s *server) newConnection(peer string, stream pb.ChatService_ChatServer) *Connection {
	return &Connection{
		Peer:        peer,
		PushChannel: make(chan *pb.Msg, 10),
		Stream:      stream,
	}
}
func (s *server) ReceivedThread(con *Connection, channel chan *pb.Msg) {
	var sender string
	for {
		msg, err := con.Stream.Recv()
		if err != nil {
			log.Println(err)
			return
		}
		sender = msg.From
		s.userMap[msg.From] = con
		if _, exist := s.bigMap[msg.From]; exist && len(s.bigMap[msg.From]) > 0 {
			for {
				select {
				case msg := <-s.bigMap[msg.From]:
					con.PushChannel <- msg
				}
			}
		}
		if msg.To == "" {
			continue
		}
		select {
		case channel <- msg:
		case <-con.Stream.Context().Done():
			delete(s.userMap, sender)
			log.Printf("%q terminated with stream closed", con.Peer)
			return
		}
	}
}

func (s *server) Chat(stream pb.ChatService_ChatServer) error {
	peerAddr := "0.0.0.0"
	ctx := stream.Context()
	if peerInfo, ok := peer.FromContext(ctx); ok {
		peerAddr = peerInfo.Addr.String()
	}
	fmt.Println(peerAddr)
	con := s.newConnection(peerAddr, stream)
	msgChan := make(chan *pb.Msg, 20)
	go s.ReceivedThread(con, msgChan)
	for {
		select {
		case msg := <-msgChan:
			if _, exist := s.userMap[msg.To]; exist {
				log.Printf("msg into pushChannel:%v", msg)
				s.userMap[msg.To].PushChannel <- msg
			} else {
				if _, exist := s.bigMap[msg.To]; !exist {
					s.bigMap[msg.To] = make(chan *pb.Msg, 1000)
				}
				log.Printf("msg into bigmap:%v", msg)
				s.bigMap[msg.To] <- msg
			}
		case msg := <-con.PushChannel:
			err := con.Stream.Send(msg)
			if err != nil {
				log.Print(err)
			}
			fmt.Printf("msg: %v:send success", msg)
		}
	}
}

func main() {
	listen, err := net.Listen("tcp", "127.0.0.1:5000")
	if err != nil {
		log.Fatalf("failed to listen")
	}
	s := grpc.NewServer(grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: 5 * time.Minute}))
	pb.RegisterChatServiceServer(s, &server{userMap: make(map[string]*Connection), bigMap: make(map[string]chan *pb.Msg)})
	s.Serve(listen)
}
