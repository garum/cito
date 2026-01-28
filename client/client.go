package main

import (
	"bufio"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	log.Println("Hello world")

	u := url.URL{Scheme: "ws", Host: "127.0.0.1:8080", Path: "/ws"}
	log.Printf("connecting to %s", u.String())
	dialer := &websocket.Dialer{HandshakeTimeout: 45 * time.Second}
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer conn.Close()

	for {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		err := conn.WriteMessage(websocket.TextMessage, []byte(text))

		if err != nil {
			log.Println(err)
			return
		}

		log.Println("send:", text)
	}
}
