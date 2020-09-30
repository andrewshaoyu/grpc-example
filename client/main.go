package main

import (
	"context"
	"flag"
	"fmt"
	pb "go-test/proto"
	"google.golang.org/grpc"
	"io"
	"log"
	"strings"
)

func sendMsg(client pb.ChatServiceClient) {
	ctx := context.TODO()
	stream, err := client.Chat(ctx)
	if err != nil {
		log.Fatal("stream error", err)
	}
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				log.Println("recv eof")
				return
			}
			if err != nil {
				log.Println("recv error", err)
				return
			}
			log.Printf(": from %s : %s", msg.From, msg.Body)
		}
	}()
	msg := pb.Msg{
		From: *from,
		To:   "",
		Body: "",
		Type: "Hello",
	}
	inputChan <- &msg
	go func() {
		for {
			select {
			case input := <-inputChan:
				err := stream.Send(input)
				if err != nil {
					log.Print(err)
				}
			}
		}
	}()
}

var from = flag.String("from", "a", "from")
var inputChan = make(chan *pb.Msg, 20)

func main() {
	flag.Parse()
	startChat()
	go func() {
		var msg pb.Msg
		msg.From = *from
		for {
			fmt.Println("please input")
			var input string
			fmt.Scanln(&input)
			inputData := strings.Split(input, "#")
			msg.To = inputData[0]
			msg.Body = inputData[1]
			inputChan <- &msg
		}
	}()
	select {}
}

func startChat() {
	conn, err := grpc.Dial("127.0.0.1:5000", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	client := pb.NewChatServiceClient(conn)
	sendMsg(client)
}
