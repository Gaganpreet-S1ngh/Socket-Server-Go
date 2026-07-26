package main

import (
	"net/http"

	"github.com/Gaganpreet-S1ngh/Socket-Server-Go/internal/platform/server"
	"golang.org/x/net/websocket"
)

func main() {
	server := server.NewServer()

	http.Handle("/ws", websocket.Server{
    Handshake: func(config *websocket.Config, req *http.Request) error {
        return nil // accept all origins (development only)
    },
    Handler: server.HandleWS,
})
	http.ListenAndServe(":7007" , nil)

}