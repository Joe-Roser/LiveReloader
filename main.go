package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var (
	rootFile  string
	port      int
	usedFiles = make(map[string]time.Time)
	upgrader  = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	socketPool = []*websocket.Conn{}
)

const (
	script = `<script>
	const socket = new WebSocket("ws://" + window.location.host + "/ws")
	socket.addEventListener("message", (event) =>{
		location.reload()
	})
	socket.addEventListener("close", (event) =>{
		console.log("Server closed socket")
	})
	</script>
	</body`
)

func printError(msg string) {
	fmt.Fprint(os.Stderr, msg)
}

func main() {
	args := os.Args
	if len(args) == 2 {
		port = 8080
	} else if len(args) == 3 {

		if parse, err := strconv.Atoi(args[2]); err != nil {
			fmt.Fprint(os.Stderr, "Error: Please enter integer as port for second argument\n")
			return
		} else {
			port = parse
		}

	} else {
		printError("Error: Please enter args - <file> <port - optional>\n")
		return
	}

	stats, err := os.Stat(args[1])
	if err != nil {
		printError("Error: File not found\n")
	}
	rootFile = args[1]
	usedFiles[rootFile] = stats.ModTime()

	fmt.Printf("Listening on http://127.0.0.1:%v\n", port)

	go updateLoop()

	// Read request
	// Parse request
	// If tcp /:
	//    Get pointed at file
	//    Insert script to get web socket
	//    respond
	// If web socket:
	//    accept
	//    handle that stuff
	// else:
	//    otherwise, serve file

	http.HandleFunc("/", handleHttp)
	http.HandleFunc("/ws", handleNewSocket)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleHttp(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if path == "/" || path == "index.html" {
		body, err := fuckWithRoot()
		if err != nil {
			msg := fmt.Sprintf("Internal Server Error: %s", err)
			http.Error(w, msg, http.StatusInternalServerError)
		}

		w.Write([]byte(body))
		return
	}

	stats, err := os.Stat(fmt.Sprintf(".%v", path))
	if err != nil {
		http.NotFoundHandler().ServeHTTP(w, r)
		return
	}
	usedFiles[path] = stats.ModTime()

	http.FileServer(http.Dir(".")).ServeHTTP(w, r)
}

func fuckWithRoot() (string, error) {
	p, err := os.ReadFile(rootFile)
	if err != nil {
		return "", err
	}
	splitRead := strings.Split(string(p), "</body")
	if len(splitRead) != 2 {
		return "", fmt.Errorf("No ending body tag found")
	}
	body := strings.Join(splitRead, script)

	return body, nil
}

func handleNewSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		msg := fmt.Sprintf("Internal server error: %s", err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	socketPool = append(socketPool, ws)
}

func updateLoop() {
	defer func() {
		for _, ws := range socketPool {
			ws.Close()
		}
	}()

	for {
		for file, time := range usedFiles {
			stat, err := os.Stat("./" + file)
			if err != nil {
				delete(usedFiles, file)
			}

			if newTime := stat.ModTime(); newTime != time {
				for _, ws := range socketPool {
					ws.WriteMessage(websocket.TextMessage, []byte("reload"))
				}
				usedFiles[file] = newTime
			}

		}

		time.Sleep(1 * time.Second)
	}

}

//
