package server

import (
	"fmt"
	"io"
	"sync"

	"golang.org/x/net/websocket"
)

type Server interface {
	HandleWS(ws *websocket.Conn)
}

type server struct {
	conns map[*websocket.Conn]bool
	mu sync.Mutex
}

func NewServer() Server {
	return &server{
		conns: make(map[*websocket.Conn]bool),
	}
}

// HandleWS implements [Server].
func (s *server) HandleWS(ws *websocket.Conn) {

	fmt.Println("New incoming connection from the client : " , ws.RemoteAddr())

	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[ws] = true

	// Readloop given to a different go routine

	go s.readLoop(ws);
}

func(s *server) readLoop(ws *websocket.Conn) {
	buf := make([]byte , 1024)

	for {
		n , err := ws.Read(buf);
		if err != nil {
			if err == io.EOF {
				break;
			}

			fmt.Println("Read error : " , err);
			return 
		}

		msg := buf[:n];
		fmt.Println(string(msg));
		ws.Write([]byte("Thank you for the message!"))
	}
}
