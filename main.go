package main

import (
	"aarjun/server"
	"log"
	"net/http"
)

const port string = ":3000"

func main() {

	staticFileServer := http.FileServer(http.Dir("./client"))

	http.Handle("/", staticFileServer)
	http.HandleFunc("/websocket", server.WebSocketHandler)

	log.Printf(server.Green + "Starting server... on http://localhost:3000" + server.Reset)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		panic(err)
	}
}
