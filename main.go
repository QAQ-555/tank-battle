package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	gamemap "example.com/lite_demo/map"
	"example.com/lite_demo/webserver"
)

func main() {
	// 加载配置
	if err := LoadConfig(); err != nil {
		log.Fatalf("无法加载配置文件: %v", err)
	}

	log.SetFlags(log.Lmicroseconds)

	webserver.InitSpawnTanks()
	http.HandleFunc(AppConfig.WebSocketPath, webserver.Handler)
	http.HandleFunc(AppConfig.MapWebSocketPath, gamemap.WsMapHandler)

	// 添加配置API
	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AppConfig)
	})

	go webserver.MapRenderloop()
	go webserver.BroadcastLoop()

	addr := fmt.Sprintf("0.0.0.0:%d", AppConfig.ServerPort)
	log.Printf("WebSocket server started on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
