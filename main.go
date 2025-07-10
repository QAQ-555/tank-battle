package main

import (
	"log"
	"net/http"

	"example.com/lite_demo/webserver"
)

func main() {
	log.SetFlags(log.Lmicroseconds)

	webserver.InitSpawnTanks()
	http.HandleFunc("/ws", webserver.Handler)

	// http.HandleFunc("/map", mapHandler)
	// http.HandleFunc("/mapws", wsMapHandler)
	go webserver.MapRenderloop()
	go webserver.BroadcastLoop()

	log.Println("WebSocket server started on :8888")
	if err := http.ListenAndServe("0.0.0.0:8881", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
