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
	// go func() {
	// 	for {
	// 		fmt.Println("========== [调试信息] ==========")

	// 		model.SpawnTanksMu.Lock()
	// 		fmt.Printf("[SpawnTanks] %+v\n", model.SpawnTanks)
	// 		model.SpawnTanksMu.Unlock()

	// 		model.ClientsMu.Lock()
	// 		fmt.Printf("[Clients] %+v\n", model.Clients)
	// 		model.ClientsMu.Unlock()

	// 		model.UsernameMu.Lock()
	// 		fmt.Printf("[Usernames] %+v\n", model.Usernames)
	// 		model.UsernameMu.Unlock()

	// 		fmt.Println("========== [调试信息结束] ==========")
	// 		time.Sleep(10 * time.Second)
	// 	}
	// }()
	log.SetFlags(log.Lmicroseconds)
	gamemap.Maprandom()
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
