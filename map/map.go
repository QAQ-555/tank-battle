package gamemap

import (
	"log"
	"net/http"
	"strings"
	"time"

	"example.com/lite_demo/model"
	"github.com/gorilla/websocket"
)

// 清空地图
func clearMap() {
	for y := 0; y < int(model.MAP_SIZE_Y); y++ {
		for x := 0; x < int(model.MAP_SIZE_X); x++ {
			model.Map[y][x] = 0
		}
	}
}

// 序列化地图
func GetMap() []byte {
	buf := make([]byte, 0, model.MAP_SIZE_X*model.MAP_SIZE_Y)
	for y := 0; y < int(model.MAP_SIZE_Y); y++ {
		buf = append(buf, model.Map[y][:]...)
	}
	return buf
}

// 在地图上标记坦克（3x3）
func MarkTankOnMap(t *model.Tank, val byte) {
	// for dx := -1; dx <= 1; dx++ {
	// 	for dy := -1; dy <= 1; dy++ {
	// 		x := int(t.LocalX) + dxX
	// 		y := int(t.LocalY) + dy
	// 		if x >= 0 && x < int(model.MAP_SIZE_X) && y >= 0 && y < int(model.MAP_SIZE_Y) {
	model.Map[t.LocalY][t.LocalX] = val
	// 		}
	// 	}
	// }
}

func getMapAsString() string {
	var sb strings.Builder
	for y := 0; y < int(model.MAP_SIZE_Y); y++ {
		for x := 0; x < int(model.MAP_SIZE_X); x++ {
			cell := model.Map[y][x]
			if cell == 0 {
				sb.WriteRune('□') // 空地
			} else {
				sb.WriteRune('■') // 占用
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func WsMapHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := model.UP.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(time.Millisecond * model.TICK_INTERVAL_MS)
	defer ticker.Stop()

	for range ticker.C {
		mapStr := getMapAsString()
		err := conn.WriteMessage(websocket.TextMessage, []byte(mapStr))
		if err != nil {
			log.Println("write map error:", err)
			return
		}
	}
}
