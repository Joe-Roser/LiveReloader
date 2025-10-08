package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type recMsg = struct {
	messageType int
	p           []byte
	err         error
	conn        *websocket.Conn
}

var msgs chan recMsg = make(chan recMsg)

func handleNewSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		msg := fmt.Sprintf("Internal server error: %s", err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	ch := ws.CloseHandler()
	ws.SetCloseHandler(func(code int, text string) error {
		for i, socket := range socketPool {
			if socket == ws {
				socketPool[i] = socketPool[len(socketPool)-1]
				socketPool = socketPool[:len(socketPool)-1]
			}
		}
		return ch(code, text)
	})

	socketPool = append(socketPool, ws)
	go sendMsgs(ws)
}

func sendMsgs(ws *websocket.Conn) {
	for {
		messageType, p, err := ws.ReadMessage()
		if err != nil {
			ws.Close()
			return
		}
		msgs <- recMsg{messageType, p, err, ws}
	}
}

func updateLoop() {
	ticker := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-ticker.C:
			fmt.Println(len(socketPool))
			for file, time := range usedFiles {
				stat, err := os.Stat("./" + file)
				if err != nil {
					delete(usedFiles, file)
				}

				if newTime := stat.ModTime(); newTime != time {
					for _, ws := range socketPool {
						ws.WriteMessage(websocket.TextMessage, []byte("reload"))
						ws.Close()
					}
					usedFiles[file] = newTime
					socketPool = []*websocket.Conn{}
				}
			}
		case msg := <-msgs:
			fmt.Println(string(msg.p))
		}
	}

}
