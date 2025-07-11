package main

import (
	"encoding/json"
	"os"
)

type Config struct {
	ServerPort       int    `json:"server_port"`
	WebSocketPath    string `json:"websocket_path"`
	MapWebSocketPath string `json:"map_websocket_path"`
}

var AppConfig Config

func LoadConfig() error {
	file, err := os.Open("config.json")
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(&AppConfig)
}
