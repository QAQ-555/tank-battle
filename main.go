package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var flagChan = make(chan bool)

const (
	MAP_SIZE_X       uint = 1542
	MAP_SIZE_Y       uint = 512
	TICK_INTERVAL_MS      = 1000
	MAP_RENDER_MS         = 1000
) //建立链接发送数据

const (
	DirUp        = 8
	DirUpRight   = 9
	DirRight     = 6
	DirDownRight = 3
	DirDown      = 2
	DirDownLeft  = 1
	DirLeft      = 4
	DirUpLeft    = 7
	DirNone      = 5
) //8向方位代码

const (
	StatusFree  byte = 0
	StatusTaken byte = 1
) //坦克状态

var UP = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
} //websocket设置

var (
	clients         = make(map[string]*Client)
	clientsMu       sync.Mutex
	spawnTanks      []*Tank
	spawnTanksMu    sync.Mutex
	activeBullets   []*Bullet
	activeBulletsMu sync.Mutex
	usernames       []string
	usernameMu      sync.Mutex
)

type WebMessage struct {
	Type byte `json:"type"`
	//TimeStamp int64       `json:"time"`
	ID      string      `json:"id"`
	Payload interface{} `json:"payload"`
}

// 发送地图信息
type MapConfig struct {
	Map          []byte `json:"map"`
	MapSizeX     uint   `json:"map_size_x"`
	MapSizeY     uint   `json:"map_size_y"`
	TickInterval int    `json:"tick_interval_ms"`
	MapRenderMS  int    `json:"map_render_ms"`
	ServerID     string `json:"username"`
}

// 坦克状态
type Tank struct {
	LocalX uint `json:"x"`
	LocalY uint `json:"y"`
	//Theta       float64 `json:theta`
	Reload      uint   `json:"reload"`
	Trigger     bool   `json:"trigger"`
	GunFacing   byte   `json:"gunfacing"`
	Status      byte   `json:"status"`
	Orientation byte   `json:"orientation"`
	ID          string `json:"id"`
}

// 游戏状态
type GameState struct {
	Tanks   []*Tank   `json:"tanks"`
	Bullets []*Bullet `json:"bullets,omitempty"`
	Map     []byte    `json:"map,omitempty"`
	//Items   []*Item   `json:"items,omitempty"`

}

// 子弹状态
type Bullet struct {
	Tank   string  `json:"shoter"`
	LocalX uint    `json:"x"`
	LocalY uint    `json:"y"`
	Facing byte    `json:"orientation"`
	Speed  float64 `json:"speed"`
}

// 客户端信息
type Client struct {
	ID         string
	Conn       *websocket.Conn
	Tank       *Tank
	LastActive time.Time
}

// 客户端请求
type OperatePayload struct {
	Up     bool
	Down   bool
	Left   bool
	Right  bool
	Action string
}

type RequestPayload struct {
	Username string `json:"username"`
	Success  bool   `json:"success"`
}

type NoticePayload struct {
	Notice string `json:"notice"`
}

// 地图数据
var Map [MAP_SIZE_Y][MAP_SIZE_X]byte

func main() {
	log.SetFlags(log.Lmicroseconds)
	initSpawnTanks()
	http.HandleFunc("/ws", handler)
	http.HandleFunc("/map", mapHandler)
	http.HandleFunc("/mapws", wsMapHandler)
	go mapRenderloop()
	go broadcastLoop()

	log.Println("WebSocket server started on :8889")
	if err := http.ListenAndServe("0.0.0.0:8889", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	conn, err := UP.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	notice := NoticePayload{
		Notice: "websocket connect success",
	}

	data, err := rePackWebMessageJson(0, notice, "perpartext")
	if err != nil {
		log.Println("Failed to marshal notice payload:", err)
		return
	}
	conn.WriteMessage(websocket.TextMessage, data)

	timeout := time.After(60 * time.Second)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()

	var username string // 记录分配的 username

	// 等待直到满足条件或超时
	if ok, u := waitForCondition(conn, tick.C, timeout); ok {
		username = u
	} else {
		log.Println("⏳ 超时，条件未达成")
		return
	}

	// 确保连接断开时清理用户名

	log.Println("执行后续逻辑...")
	// 后续逻辑...
	tank := allocateTank()
	if tank == nil {
		log.Printf("No available spawn point for %s\n", username)
		data, err := rePackWebMessageJson(0, []byte("No available spawn point"), username)
		if err != nil {
			log.Println("Failed to marshal game state:", err)
			return
		}
		conn.WriteMessage(websocket.TextMessage, data)
		conn.Close()
		return
	}

	client := &Client{
		ID:         username,
		Conn:       conn,
		Tank:       tank,
		LastActive: time.Now(),
	}
	client.Tank.ID = client.ID //每个链接占用一个坦克
	clientsMu.Lock()
	clients[username] = client
	clientsMu.Unlock()
	sendConfig(conn, client.ID)
	log.Printf("New connection: %s at (%d,%d) facing %d\n",
		username, tank.LocalX, tank.LocalY, tank.Orientation)
	printTankShape(tank)
	go readMessages(client)
}

// 处理客户端指令
func readMessages(client *Client) {
	defer func() {
		client.Conn.Close()
		clientsMu.Lock()
		delete(clients, client.ID)
		clientsMu.Unlock()
		removeUsername(client.ID)
		if client.Tank != nil {
			freeTank(client.Tank)
			log.Printf("Freed spawn for %s\n", client.ID)
		}

		log.Printf("Connection %s closed\n", client.ID)
	}()

	for {
		_, msg, err := client.Conn.ReadMessage()
		if err != nil {
			log.Printf("test Connection %s error: %v\n", client.ID, err)
			break
		}

		_, _, payload, err := UnpackWebMessage(msg)
		if err != nil {
			log.Printf("Failed to parse JSON from %s: %v", client.ID, err)
			continue
		}
		if op, ok := payload.(OperatePayload); ok {
			moveDir := parseDirection(op.Up, op.Down, op.Left, op.Right)

			client.LastActive = time.Now()
			client.Tank.Orientation = moveDir

			if moveDir != DirNone {
				client.Tank.GunFacing = moveDir
			}
			log.Printf("tank %s try move to %d", client.ID, client.Tank.Orientation)
			if op.Action == "fire" && client.Tank.Reload == 0 { //接收到开火命令且已经装填完毕后将扳机置于开
				log.Printf("tank %s try fire and already Reload", client.ID)
				client.Tank.Trigger = true
			}
			printTankShape(client.Tank)
		} else {
			log.Printf("payload 不是 OperatePayload，而是：%T", payload)
		}

		//moveTank(client.Tank, moveDir)
	}
}

// 更新游戏状态
func mapRenderloop() {
	ticker := time.NewTicker(MAP_RENDER_MS * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// 遍历坦克，把每个活跃的坦克标记到地图上

		spawnTanksMu.Lock()
		activeBulletsMu.Lock()
		for _, t := range spawnTanks {
			//坦克移动
			if t.Status == StatusTaken {
				markTankOnMap(t, 0)
				moveTank(t)
				// if t.Trigger { //更新坦克状态时，如果坦克扳机按下则发射子弹
				// 	activeBullets = append(activeBullets, openFire(t))
				// }
			}
			//发射子弹
		}
		// for _, b := range activeBullets {

		// }
		spawnTanksMu.Unlock()
		activeBulletsMu.Unlock()
		flagChan <- true
	}

}

// 服务器广播地图
func broadcastLoop() {
	ticker := time.NewTicker(TICK_INTERVAL_MS * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		BroadcastGameState()
		<-flagChan
	}
}

// 广播地图状态
func BroadcastGameState() {
	state := BuildGameState()
	data, err := rePackWebMessageJson(2, state, "broadcast message gamer")
	if err != nil {
		log.Println("Failed to marshal game state:", err)
		return
	}
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for _, c := range clients {
		if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending to %s: %v\n", c.ID, err)
		}
	}
}

// 开火生成子弹
func openFire(t *Tank) *Bullet {
	var bullet Bullet
	bullet.Facing = t.GunFacing
	bullet.Speed = 2
	bullet.Tank = t.ID
	switch t.GunFacing {
	case DirDown:
		bullet.LocalX = t.LocalX
		bullet.LocalY = t.LocalY + 2
	case DirUp:
		bullet.LocalX = t.LocalX
		bullet.LocalY = t.LocalY - 2
	case DirLeft:
		bullet.LocalX = t.LocalX - 2
		bullet.LocalY = t.LocalY
	case DirRight:
		bullet.LocalX = t.LocalX + 2
		bullet.LocalY = t.LocalY
	case DirUpLeft:
		bullet.LocalX = t.LocalX - 2
		bullet.LocalY = t.LocalY - 2
	case DirUpRight:
		bullet.LocalX = t.LocalX + 2
		bullet.LocalY = t.LocalY - 2
	case DirDownLeft:
		bullet.LocalX = t.LocalX - 2
		bullet.LocalY = t.LocalY + 2
	case DirDownRight:
		bullet.LocalX = t.LocalX + 2
		bullet.LocalY = t.LocalY + 2
	default:
		// 如果方向未知，就放在坦克正中央
		bullet.LocalX = t.LocalX
		bullet.LocalY = t.LocalY
	}
	t.Reload = 500
	t.Trigger = false
	printTankShape(t)
	log.Printf("shoting bullet=%+v\n", bullet)
	return &bullet
}

// 清空地图
func clearMap() {
	for y := 0; y < int(MAP_SIZE_Y); y++ {
		for x := 0; x < int(MAP_SIZE_X); x++ {
			Map[y][x] = 0
		}
	}
}

// 控制台打印坦克信息
func printTankShape(t *Tank) {
	dirSymbols := map[byte]string{
		DirUp:        "↑",
		DirUpRight:   "↗",
		DirRight:     "→",
		DirDownRight: "↘",
		DirDown:      "↓",
		DirDownLeft:  "↙",
		DirLeft:      "←",
		DirUpLeft:    "↖",
		DirNone:      "o",
	}

	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				fmt.Print(dirSymbols[t.Orientation])
			} else {
				fmt.Print("#")
			}
		}
		fmt.Println()
	}
	fmt.Println(t)
}

// 指令转化为方向
func parseDirection(up, down, left, right bool) byte {
	switch {
	case up && left && !down && !right:
		return DirUpLeft
	case up && right && !down && !left:
		return DirUpRight
	case down && left && !up && !right:
		return DirDownLeft
	case down && right && !up && !left:
		return DirDownRight
	case up && !down:
		return DirUp
	case down && !up:
		return DirDown
	case left && !right:
		return DirLeft
	case right && !left:
		return DirRight
	default:
		return DirNone
	}
}

// 移动坦克
func moveTank(t *Tank) {
	// spawnTanksMu.Lock()
	// defer spawnTanksMu.Unlock()

	// 清除当前地图上的标记
	//markTankOnMap(t, 0)

	// 计算目标位置
	dx, dy := 0, 0
	switch t.Orientation {
	case DirUp:
		dy = -1
	case DirUpRight:
		dx, dy = 1, -1
	case DirRight:
		dx = 1
	case DirDownRight:
		dx, dy = 1, 1
	case DirDown:
		dy = 1
	case DirDownLeft:
		dx, dy = -1, 1
	case DirLeft:
		dx = -1
	case DirUpLeft:
		dx, dy = -1, -1
	}

	newX := int(t.LocalX) + dx
	newY := int(t.LocalY) + dy

	// 检查是否越界
	if newX-1 < 0 || newX+1 >= int(MAP_SIZE_X) || newY-1 < 0 || newY+1 >= int(MAP_SIZE_Y) {
		log.Printf("Tank at (%d,%d) cannot move %v — out of bounds", t.LocalX, t.LocalY, t.Orientation)
		//markTankOnMap(t, 1) // 还原
		return
	}

	// 检查目标区域是否被占用
	for x := newX - 1; x <= newX+1; x++ {
		for y := newY - 1; y <= newY+1; y++ {
			if Map[y][x] != 0 {
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

// 地图展示
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

let ws = new WebSocket("ws://" + 192.168.10.123:8888 + "/mapws");
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
	conn, err := UP.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(time.Millisecond * TICK_INTERVAL_MS)
	defer ticker.Stop()
	sendConfig(conn, "viewer")
	for range ticker.C {
		state := BuildGameState()
		data, err := rePackWebMessageJson(2, state, "broadcast message viewer")
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

// 链接建立时 发送所需数据
func sendConfig(conn *websocket.Conn, id string) {

	config := MapConfig{
		Map:          GetMap(),
		MapSizeX:     MAP_SIZE_X,
		MapSizeY:     MAP_SIZE_Y,
		TickInterval: TICK_INTERVAL_MS,
		MapRenderMS:  MAP_RENDER_MS,
		ServerID:     id,
	}
	data, err := rePackWebMessageJson(1, config, id)
	if err != nil {
		log.Println("Failed to marshal game state:", err)
		return
	}
	err = conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		log.Println("write message error:", err)
		return
	}
}

// 获取所有活动中坦克
func GetActiveTanks() []*Tank {
	spawnTanksMu.Lock()
	defer spawnTanksMu.Unlock()

	var active []*Tank
	for _, t := range spawnTanks {
		if t.Status != StatusFree {
			active = append(active, t)
		}
	}
	return active
}

// 序列化地图
func GetMap() []byte {
	buf := make([]byte, 0, MAP_SIZE_X*MAP_SIZE_Y)
	for y := 0; y < int(MAP_SIZE_Y); y++ {
		buf = append(buf, Map[y][:]...)
	}
	return buf
}

// 构建游戏状态结构体
func BuildGameState() *GameState {
	return &GameState{
		Tanks:   GetActiveTanks(),
		Bullets: activeBullets,
		// Items: GetActiveItems(),
		// Map: GetMap(),
	}
}

// 在地图上标记坦克（3x3）
func markTankOnMap(t *Tank, val byte) {
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			x := int(t.LocalX) + dx
			y := int(t.LocalY) + dy
			if x >= 0 && x < int(MAP_SIZE_X) && y >= 0 && y < int(MAP_SIZE_Y) {
				Map[y][x] = val
			}
		}
	}
}

// 初始化出生点
func initSpawnTanks() {

	coords := [][2]uint{
		{1, 1},                           // 上左
		{MAP_SIZE_X / 2, 1},              // 上中
		{MAP_SIZE_X - 2, 1},              // 上右
		{MAP_SIZE_X - 2, MAP_SIZE_Y / 2}, // 右中
		{MAP_SIZE_X - 2, MAP_SIZE_Y - 2}, // 下右
		{MAP_SIZE_X / 2, MAP_SIZE_Y - 2}, // 下中
		{1, MAP_SIZE_Y - 2},              // 下左
		{1, MAP_SIZE_Y / 2},              // 左中
	}

	for _, c := range coords {
		x, y := c[0], c[1]
		tank := &Tank{
			LocalX:      x,
			LocalY:      y,
			Reload:      0,
			Trigger:     false,
			Status:      StatusFree,
			GunFacing:   DirDown,
			Orientation: DirNone,
		}
		spawnTanks = append(spawnTanks, tank)
	}
}

// 分配出生点
func allocateTank() *Tank {
	spawnTanksMu.Lock()
	defer spawnTanksMu.Unlock()

	for _, t := range spawnTanks {
		if t.Status == StatusFree {
			t.Status = StatusTaken
			markTankOnMap(t, 1)
			return t
		}
	}
	return nil
}

// 释放出生点
func freeTank(t *Tank) {
	spawnTanksMu.Lock()
	defer spawnTanksMu.Unlock()

	t.Status = StatusFree
	markTankOnMap(t, 0)
}

func rePackWebMessageJson(msgType byte, payload interface{}, id string) ([]byte, error) {
	mes := WebMessage{
		Type: msgType,
		//TimeStamp: time.Now().UnixNano(),
		ID:      id,
		Payload: payload,
	}
	return json.Marshal(mes)
}

func UnpackWebMessage(data []byte) (byte, string, interface{}, error) {
	var mes WebMessage
	err := json.Unmarshal(data, &mes)
	if err != nil {
		return 0, "", nil, err
	}

	// 因为 Payload 是 interface{}，它现在是 map[string]interface{}
	// 所以我们先把它再 Marshal 一次，得到原始 JSON
	payloadBytes, err := json.Marshal(mes.Payload)
	if err != nil {
		return 0, "", nil, err
	}

	var payload interface{}

	switch mes.Type {
	case 15:
		var op OperatePayload
		if err := json.Unmarshal(payloadBytes, &op); err != nil {
			return 0, "", nil, err
		}
		payload = op
		//log.Printf("%+v", op)
		//log.Printf("%+v", payload)
	case 16:
		var tp RequestPayload
		if err := json.Unmarshal(payloadBytes, &tp); err != nil {
			return 0, "", nil, err
		}
		payload = tp
	default:
		return 0, "", nil, fmt.Errorf("unknown message type: %d", mes.Type)
	}

	return mes.Type, mes.ID, payload, nil
}

//func bullet
// 生成地图快照字符串
// func buildMapSnapshot() string {
// 	var snapshot string
// 	for y := 0; y < int(MAP_SIZE_Y); y++ {
// 		for x := 0; x < int(MAP_SIZE_X); x++ {
// 			if Map[y][x] == 0 {
// 				snapshot += "□"
// 			} else {
// 				snapshot += "■"
// 			}
// 		}
// 		snapshot += "\n"
// 	}
// 	return snapshot
// }

func waitForCondition(conn *websocket.Conn, tick <-chan time.Time, timeout <-chan time.Time) (bool, string) {
	for {
		select {
		case <-timeout:
			return false, ""

		case <-tick:
			ok, err, username := checkCondition(conn)
			if err != nil {
				log.Printf("读取或解析出错: %v", err)
				continue
			}
			if ok {
				return true, username
			}
		}
	}
}

func checkCondition(conn *websocket.Conn) (bool, error, string) {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return false, fmt.Errorf("connection error: %w", err), ""
	}

	_, _, payload, err := UnpackWebMessage(msg)
	if err != nil {
		return false, fmt.Errorf("failed to parse message: %w", err), ""
	}

	rp, ok := payload.(RequestPayload)
	if !ok {
		log.Printf("payload 不是 RequestPayload，而是：%T", payload)
		return false, nil, ""
	}

	if rp.Success && isUsernameLegal(rp.Username) {
		usernameMu.Lock()
		usernames = append(usernames, rp.Username)
		usernameMu.Unlock()
		return true, nil, rp.Username
	} else {
		notice := NoticePayload{
			Notice: "username is empty or already exists",
		}

		data, err := rePackWebMessageJson(0, notice, rp.Username)
		if err != nil {
			log.Println("Failed to marshal notice payload:", err)
			return false, nil, ""
		}
		conn.WriteMessage(websocket.TextMessage, data)
	}
	return false, nil, ""
}

// 从 usernames 切片里删除 username
func removeUsername(username string) {
	usernameMu.Lock()
	defer usernameMu.Unlock()

	newList := make([]string, 0, len(usernames))
	for _, u := range usernames {
		if u != username {
			newList = append(newList, u)
		}
	}
	usernames = newList

	log.Printf("已删除用户名: %s", username)
}

func isUsernameLegal(username string) bool {
	// 先判断 username 是否为空
	if username == "" {
		return false
	}

	// 遍历现有的 usernames
	for _, u := range usernames {
		if u == "" {
			continue
		}
		if u == username {
			// 找到相同的
			return false
		}
	}

	// 非空且不重复
	return true
}
