package gamemap

import (
	"fmt"
	"log"
	"net/http"
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
	// 		x := int(t.LocalX) + dx
	// 		y := int(t.LocalY) + dy
	// 		if x >= 0 && x < int(model.MAP_SIZE_X) && y >= 0 && y < int(model.MAP_SIZE_Y) {
	model.Map[t.LocalY][t.LocalY] = val
	// 		}
	// 	}
	// }
}

func moveTank(t *model.Tank) {
	// SpawnTanksMu.Lock()
	// defer SpawnTanksMu.Unlock()

	// 清除当前地图上的标记
	//markTankOnMap(t, 0)

	// 计算目标位置
	dx, dy := 0, 0
	switch t.Orientation {
	case model.DirUp:
		dy = -1
	case model.DirUpRight:
		dx, dy = 1, -1
	case model.DirRight:
		dx = 1
	case model.DirDownRight:
		dx, dy = 1, 1
	case model.DirDown:
		dy = 1
	case model.DirDownLeft:
		dx, dy = -1, 1
	case model.DirLeft:
		dx = -1
	case model.DirUpLeft:
		dx, dy = -1, -1
	}

	newX := int(t.LocalX) + dx
	newY := int(t.LocalY) + dy

	// 检查是否越界
	if newX-1 < 0 || newX+1 >= int(model.MAP_SIZE_X) || newY-1 < 0 || newY+1 >= int(model.MAP_SIZE_Y) {
		log.Printf("Tank at (%d,%d) cannot move %v — out of bounds", t.LocalX, t.LocalY, t.Orientation)
		//markTankOnMap(t, 1) // 还原
		return
	}

	// 检查目标区域是否被占用
	for x := newX - 1; x <= newX+1; x++ {
		for y := newY - 1; y <= newY+1; y++ {
			if model.Map[y][x] != 0 {
				log.Printf("Tank at (%d,%d) cannot move %v — blocked", t.LocalX, t.LocalY, t.Orientation)
				//destroyTank()
				return
			}
		}
	}
	//地图更新后，发生重合 坦克炮弹直接摧毁自身
	// 更新位置
	t.LocalX = uint(newX)
	t.LocalY = uint(newY)

	// 标记新位置
	//markTankOnMap(t, 1)

	log.Printf("tank %s moved to (%d,%d) facing %d", t.ID, t.LocalX, t.LocalY, t.Orientation)
}
func mapHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `
<html>
<head>
<meta charset="UTF-8">
<title>Map Viewer</title>
<style>
  body {
    background: white;
    color: black;
    font-family: monospace;
    white-space: pre;
    font-size: 12px;
  }
</style>
</head>
<body>
<pre id="map">Loading map...</pre>

<script>
let mapData = null;
let mapSizeX = 0;
let mapSizeY = 0;
let tanks = [];
const mapElem = document.getElementById("map");

function renderMap() {
    if (!mapData) {
        mapElem.textContent = "No map data yet.";
        return;
    }

    let output = "";
    for (let y = 0; y < mapSizeY; y++) {
        for (let x = 0; x < mapSizeX; x++) {
            const idx = y * mapSizeX + x;
            const cell = mapData[idx];
            if (cell === 1) {
                output += "T";  // 坦克/障碍
            } else {
                output += ".";  // 空地
            }
        }
        output += "\n";
    }
    mapElem.textContent = output;
}

function updateTanks(tankList) {
    // 先清空地图上的坦克位
    for (let i = 0; i < mapData.length; i++) {
        if (mapData[i] === 1) mapData[i] = 0;
    }
    // 标记坦克
    tankList.forEach(t => {
        if (t.x >= 0 && t.x < mapSizeX && t.y >= 0 && t.y < mapSizeY) {
            mapData[t.y * mapSizeX + t.x] = 1;
        }
    });
    renderMap();
}

let ws = new WebSocket("ws://" + location.host + "/mapws");
ws.binaryType = "arraybuffer"; // 关键

ws.onmessage = function(event) {
    if (typeof event.data === "string") {
        const data = JSON.parse(event.data);

        if (data.map_size_x && data.map_size_y) {
            mapSizeX = data.map_size_x;
            mapSizeY = data.map_size_y;
            // 等待后续二进制地图数据
        } else if (data.tanks) {
            updateTanks(data.tanks);
        }
    } else if (event.data instanceof ArrayBuffer) {
        // 接收地图字节数组
        mapData = new Uint8Array(event.data);
        renderMap();
    }
};

ws.onclose = function() {
    mapElem.textContent = "WebSocket closed";
};
</script>
</body>
</html>
`)
}

// 向预览网页广播
func wsMapHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := model.UP.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(time.Millisecond * model.TICK_INTERVAL_MS)
	defer ticker.Stop()
	model.sendConfig(conn, "viewer")
	for range ticker.C {
		state := BuildGameState()
		data, err := rePackWebMessageJson(3, state, "broadcast message viewer")
		if err != nil {
			log.Println("Failed to marshal game state:", err)
			return
		}
		err = conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Println("write error:", err)
			return
		}
	}
}
