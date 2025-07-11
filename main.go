package main

import (
	"log"
	"net/http"

	gamemap "example.com/lite_demo/map"
	"example.com/lite_demo/webserver"
)

func main() {
	log.SetFlags(log.Lmicroseconds)

	webserver.InitSpawnTanks()
	http.HandleFunc("/ws", webserver.Handler)

	//http.HandleFunc("/map", mapHandler)
	http.HandleFunc("/mapws", gamemap.WsMapHandler)
	go webserver.MapRenderloop()
	go webserver.BroadcastLoop()

	log.Println("WebSocket server started on :8888")
	if err := http.ListenAndServe("0.0.0.0:8887", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
