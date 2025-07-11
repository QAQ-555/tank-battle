package webserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	gamemap "example.com/lite_demo/map"
	"example.com/lite_demo/model"
	"github.com/gorilla/websocket"
)

type RequestHandler interface {
	RequestHandler(client *model.Client)
}

// 处理链接请求
// 处理链接请求
func Handler(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接为 WebSocket
	conn, err := model.UP.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// 向客户端发送连接成功通知
	notice := model.NoticePayload{
		Notice: "websocket connect success",
	}
	data, err := RePackWebMessageJson(0, notice, "perpartext")

	if err != nil {
		log.Println("Failed to marshal notice payload:", err)
		return
	}
	conn.WriteMessage(websocket.TextMessage, data)

	// 等待客户端提交用户名
	var username string
	ok, username := WaitForCondition(conn)
	if ok {
		log.Println("✅ 成功获取 username:", username)
	} else {
		log.Println("⏳ 超时或失败，未获取 username")
		conn.Close() // 关闭连接，释放资源
		return
	}
	// username = uuid.NewString()
	// 为客户端分配坦克
	tank := allocateTank()
	if tank == nil {
		log.Printf("❌ No available spawn point for %s\n", username)

		data, err := RePackWebMessageJson(4, []byte("No available spawn point"), username)
		if err != nil {
			log.Println("Failed to marshal game state:", err)
			return
		}
		conn.WriteMessage(websocket.TextMessage, data)
		conn.Close()
		removeUsername(username)
		return
	}

	// 注册客户端
	client := &model.Client{
		ID:         username,
		Conn:       conn,
		Tank:       tank,
		LastActive: time.Now(),
	}
	client.Tank.ID = client.ID // 每个连接占用一个坦克

	model.ClientsMu.Lock()
	model.Clients[username] = client
	model.ClientsMu.Unlock()

	// 发送地图配置信息
	SendConfig(conn, client.ID, tank)

	log.Printf("🎮 New connection: %s at (%d,%d) facing %d\n",
		username, tank.LocalX, tank.LocalY, tank.Orientation)

	//printTankShape(tank)

	// 启动客户端消息读取 goroutine
	go readMessages(client)
}

// 处理客户端指令
func readMessages(client *model.Client) {
	defer cleanUpClient(client)

	for {
		_, msg, err := client.Conn.ReadMessage()
		if err != nil {
			log.Printf("⚠️ Connection %s error: %v\n", client.ID, err)
			break
		}

		_, _, payload, err := UnpackWebMessage(msg)
		if err != nil {
			log.Printf("❌ Failed to parse JSON from %s: %v", client.ID, err)
			continue
		}

		handlePayload(client, payload)
	}
}

func cleanUpClient(client *model.Client) {
	client.Conn.Close()
	model.ClientsMu.Lock()
	delete(model.Clients, client.ID)
	model.ClientsMu.Unlock()
	removeUsername(client.ID)

	if client.Tank != nil {
		FreeTank(client.Tank)
		log.Printf("✅ Freed spawn for %s\n", client.ID)
	}

	log.Printf("🔌 Connection %s closed\n", client.ID)
}

func handlePayload(client *model.Client, payload any) {
	switch p := payload.(type) {
	case model.OperatePayload:
		handleOperatePayload(client, p)
	case model.HitPayload:
		handleHitPayload(client, p)
	default:
		log.Printf("⚠️ Unknown payload type: %T", payload)
	}
}

func handleOperatePayload(client *model.Client, op model.OperatePayload) {
	moveDir := parseDirection(op.Up, op.Down, op.Left, op.Right)
	log.Println(op)
	client.LastActive = time.Now()
	client.Tank.Orientation = moveDir

	if moveDir != model.DirNone {
		client.Tank.GunFacing = moveDir
	}

	if op.Action == "fire" && client.Tank.Reload == 0 {
		log.Printf("🔥 tank %s fires (reload OK)", client.ID)
		se := OpenFire(client.Tank)

		data, err := RePackWebMessageJson(3, se, "broadcast message gamer")
		if err != nil {
			log.Println("Failed to marshal fire event:", err)
			return
		}

		broadcastToAll(data)
	}
}

func handleHitPayload(client *model.Client, hit model.HitPayload) {
	data, err := RePackWebMessageJson(7, hit, "broadcast message gamer")
	if err != nil {
		log.Println("Failed to marshal hit event:", err)
		return
	}
	log.Println("HitPayload")
	broadcastToAll(data)

	for _, t := range model.SpawnTanks {
		if t.ID == hit.Victim {
			FreeTank(t)
			removeUsername(t.ID)
		}
	}
}

func broadcastToAll(data []byte) {
	model.ClientsMu.Lock()
	defer model.ClientsMu.Unlock()
	for _, c := range model.Clients {
		if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending to %s: %v\n", c.ID, err)
		}
	}
}

func BroadcastLoop() {
	ticker := time.NewTicker(model.TICK_INTERVAL_MS * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		BroadcastGameState()
	}
}

// 广播地图状态
func BroadcastGameState() {
	state := BuildGameState()
	data, err := RePackWebMessageJson(2, state, "broadcast message gamer")
	if err != nil {
		log.Println("Failed to marshal game state:", err)
		return
	}

	var disconnectedClients []string

	model.ClientsMu.Lock()
	for _, c := range model.Clients {
		if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending to %s: %v\n", c.ID, err)
			disconnectedClients = append(disconnectedClients, c.ID)
		}
	}

	// 移除断开连接的客户端并清理资源
	for _, id := range disconnectedClients {
		if client, exists := model.Clients[id]; exists {
			// 清理坦克资源
			if client.Tank != nil {
				FreeTank(client.Tank)
			}
			// 从用户名注册表中移除
			removeUsername(id)
			// 关闭连接
			client.Conn.Close()
			// 从客户端映射中删除
			delete(model.Clients, id)
			log.Printf("Removed disconnected client: %s\n", id)
		}
	}
	model.ClientsMu.Unlock()
}

// 链接建立时 发送所需数据
func SendConfig(conn *websocket.Conn, id string, t *model.Tank) {

	config := model.MapConfig{
		Map:          gamemap.GetMap(),
		MapSizeX:     model.MAP_SIZE_X,
		MapSizeY:     model.MAP_SIZE_Y,
		TickInterval: model.TICK_INTERVAL_MS,
		MapRenderMS:  model.MAP_RENDER_MS,
		TankCoordX:   t.LocalX,
		TankCoordY:   t.LocalY,
		Tankfacing:   t.GunFacing,
		ServerID:     id,
	}
	data, err := RePackWebMessageJson(1, config, id)
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

// 打包为webmessage
func RePackWebMessageJson(msgType byte, payload interface{}, id string) ([]byte, error) {
	mes := model.WebMessage{
		Type: msgType,
		//TimeStamp: time.Now().UnixNano(),
		ID:      id,
		Payload: payload,
	}
	return json.Marshal(mes)
}

// 将webmessage解包取得payload
func UnpackWebMessage(data []byte) (byte, string, interface{}, error) {
	var mes model.WebMessage
	err := json.Unmarshal(data, &mes)
	if err != nil {
		return 0, "", nil, err
	}
	payloadBytes, err := json.Marshal(mes.Payload)
	if err != nil {
		return 0, "", nil, err
	}

	var payload interface{}

	switch mes.Type {
	case 15:
		var op model.OperatePayload
		if err := json.Unmarshal(payloadBytes, &op); err != nil {
			return 0, "", nil, err
		}
		payload = op
	case 16:
		var tp model.RequestPayload
		if err := json.Unmarshal(payloadBytes, &tp); err != nil {
			return 0, "", nil, err
		}
		payload = tp
	case 17:
		var tp model.HitPayload
		if err := json.Unmarshal(payloadBytes, &tp); err != nil {
			return 0, "", nil, err
		}
		payload = tp
	default:
		return 0, "", nil, fmt.Errorf("unknown message type: %d", mes.Type)
	}
	return mes.Type, mes.ID, payload, nil
}

func WaitForCondition(conn *websocket.Conn) (bool, string) {
	// log.Println("[WaitForCondition] 开始")

	conn.SetReadDeadline(time.Now().Add(model.WAIT_REPLY_TIME * time.Second))
	msgCh := make(chan []byte)
	timeoutCh := make(chan bool)
	closeCh := make(chan bool)

	// 读取消息的 goroutine
	go func() {
		log.Println("[goroutine] 启动")
		defer log.Println("[goroutine] 退出")

		for {
			// log.Println("[goroutine] 开始 ReadMessage")
			_, msg, err := conn.ReadMessage()
			if err != nil {
				// log.Println("[goroutine] ReadMessage 出错:", err)

				// log.Println("[goroutine] 尝试写入 timeoutCh")
				timeoutCh <- true
				// log.Println("[goroutine] 写入 timeoutCh 成功")
				// log.Println("[goroutine] 等待从 closeCh 读取以完成同步")
				<-closeCh
				// log.Println("[goroutine] 收到 closeCh:", val)
				return
			}

			// log.Println("[goroutine] 读到消息:", string(msg))

			// log.Println("[goroutine] 尝试写入 msgCh")
			msgCh <- msg
			// log.Println("[goroutine] 写入 msgCh 成功")

			// log.Println("[goroutine] 等待从 closeCh 读取")
			val := <-closeCh
			// log.Println("[goroutine] 从 closeCh 收到:", val)
			if val {
				// log.Println("[goroutine] 收到 true，退出")
				return
			}
			// log.Println("[goroutine] 收到 false，继续循环")
		}
	}()

	defer func() {
		// log.Println("[WaitForCondition] defer: 关闭 closeCh")
		conn.SetReadDeadline(time.Time{})
		close(closeCh)
	}()

	for {
		// log.Println("[WaitForCondition] 等待 select")
		select {
		case <-timeoutCh:
			// log.Println("[WaitForCondition] 从 timeoutCh 收到信号")
			// log.Println("[WaitForCondition] 尝试写入 closeCh")
			closeCh <- true
			// log.Println("[WaitForCondition] 写入 closeCh 完成")
			return false, "timeout"

		case msg := <-msgCh:
			// log.Println("[WaitForCondition] 从 msgCh 收到:", string(msg))
			readNext, username, err := processMessage(conn, msg)
			// log.Println("[WaitForCondition] processMessage 返回:", readNext, username, err)

			// log.Println("[WaitForCondition] 尝试写入 closeCh:", readNext)
			closeCh <- readNext
			// log.Println("[WaitForCondition] 写入 closeCh 完成")

			if readNext {
				// log.Println("[WaitForCondition] 条件满足，返回 true,", username)
				return true, username
			} else {
				// log.Println("[WaitForCondition] 条件未满足，继续等待")
				if err != nil {
					// log.Println("[WaitForCondition] processMessage 错误:", err)
				}
			}
		}
	}
}

// 处理信息
func processMessage(conn *websocket.Conn, msg []byte) (bool, string, error) {
	_, _, payload, err := UnpackWebMessage(msg)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse message: %w", err)
	}

	rp, ok := payload.(model.RequestPayload)
	if !ok {
		log.Printf("payload 不是 RequestPayload，而是：%T", payload)
		return false, "", nil
	}

	if rp.Success && isUsernameLegal(rp.Username) {
		model.UsernameMu.Lock()
		model.Usernames = append(model.Usernames, rp.Username)
		model.UsernameMu.Unlock()
		return true, rp.Username, nil
	}

	notice := model.NoticePayload{
		Notice: "username is empty or already exists",
	}

	data, err := RePackWebMessageJson(4, notice, rp.Username)
	if err != nil {
		log.Println("Failed to marshal notice payload:", err)
		return false, "", nil
	}
	conn.WriteMessage(websocket.TextMessage, data)

	return false, "", nil
}
