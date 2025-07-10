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

// 处理链接请求
func Handler(w http.ResponseWriter, r *http.Request) {
	//升级http
	conn, err := model.UP.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	//发送请求
	notice := model.NoticePayload{
		Notice: "websocket connect success",
	}

	data, err := RePackWebMessageJson(0, notice, "perpartext")
	if err != nil {
		log.Println("Failed to marshal notice payload:", err)
		return
	}
	conn.WriteMessage(websocket.TextMessage, data)
	//倒计时请求username

	var username string // 记录分配的 username

	ok, username := WaitForCondition(conn)
	if ok {
		log.Println("成功获取 username:", username)
	} else {
		log.Println("⏳ 超时或失败，条件未达成")
		conn.Close() // 关闭连接，确保释放资源
		return
	}
	//后续逻辑...
	tank := allocateTank()
	if tank == nil {
		log.Printf("No available spawn point for %s\n", username)
		data, err := RePackWebMessageJson(0, []byte("No available spawn point"), username)
		if err != nil {
			log.Println("Failed to marshal game state:", err)
			return
		}
		conn.WriteMessage(websocket.TextMessage, data)
		conn.Close()
		return
	}

	client := &model.Client{
		ID:         username,
		Conn:       conn,
		Tank:       tank,
		LastActive: time.Now(),
	}
	client.Tank.ID = client.ID //每个链接占用一个坦克
	model.ClientsMu.Lock()
	model.Clients[username] = client
	model.ClientsMu.Unlock()
	sendConfig(conn, client.ID, tank)
	log.Printf("New connection: %s at (%d,%d) facing %d\n",
		username, tank.LocalX, tank.LocalY, tank.Orientation)
	printTankShape(tank)
	go readMessages(client)
}

// 处理客户端指令
func readMessages(client *model.Client) {
	defer func() {
		client.Conn.Close()
		model.ClientsMu.Lock()
		delete(model.Clients, client.ID)
		model.ClientsMu.Unlock()
		removeUsername(client.ID)
		if client.Tank != nil {
			freeTank(client.Tank)
			log.Printf("Freed spawn for %s\n", client.ID)
		}

		log.Printf("Connection %s closed\n", client.ID)
	}()

	for {
		_, msg, err := client.Conn.ReadMessage()
		fmt.Printf("**********************************************")
		if err != nil {
			log.Printf("Connection %s error: %v\n", client.ID, err)
			break
		}

		_, _, payload, err := UnpackWebMessage(msg)
		if err != nil {
			log.Printf("Failed to parse JSON from %s: %v", client.ID, err)
			continue
		}
		if op, ok := payload.(model.OperatePayload); ok {
			moveDir := parseDirection(op.Up, op.Down, op.Left, op.Right)

			client.LastActive = time.Now()
			client.Tank.Orientation = moveDir

			if moveDir != model.DirNone {
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
	}
}

// 广播地图
func BroadcastLoop() {
	ticker := time.NewTicker(model.TICK_INTERVAL_MS * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		BroadcastGameState()
		<-model.FlagChan
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
	model.ClientsMu.Lock()
	defer model.ClientsMu.Unlock()
	for _, c := range model.Clients {
		if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending to %s: %v\n", c.ID, err)
		}
	}
}

// 链接建立时 发送所需数据
func sendConfig(conn *websocket.Conn, id string, t *model.Tank) {

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

	// 因为 Payload 是 interface{}，它现在是 map[string]interface{}
	// 所以我们先把它再 Marshal 一次，得到原始 JSON
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
		//log.Printf("%+v", op)
		//log.Printf("%+v", payload)
	case 16:
		var tp model.RequestPayload
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

	data, err := RePackWebMessageJson(0, notice, rp.Username)
	if err != nil {
		log.Println("Failed to marshal notice payload:", err)
		return false, "", nil
	}
	conn.WriteMessage(websocket.TextMessage, data)

	return false, "", nil
}
