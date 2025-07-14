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
// 处理链接请求
func Handler(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接为 WebSocket
	conn, err := model.UP.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// 注册客户端
	client := &model.Client{
		Conn:       conn,
		LastActive: time.Now(),
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
	log.Printf("lock 1")
	client.WriteMutex.Lock()
	conn.WriteMessage(websocket.TextMessage, data)
	client.WriteMutex.Unlock()
	log.Printf("unlock 1")

	// 等待客户端提交用户名
	var username string
	ok, username := WaitForCondition(client)
	if ok {
		log.Println("✅ 成功获取 username:", username)
	} else {
		log.Println("⏳ 超时或失败，未获取 username")
		conn.Close() // 关闭连接，释放资源
		return
	}
	client.ID = username
	// 为客户端分配坦克
	tank := allocateTank()
	if tank == nil {
		log.Printf("❌ No available spawn point for %s\n", username)

		data, err := RePackWebMessageJson(4, []byte("No available spawn point"), username)
		if err != nil {
			log.Println("Failed to marshal game state:", err)
			return
		}
		log.Printf("lock 2")
		client.WriteMutex.Lock()
		conn.WriteMessage(websocket.TextMessage, data)
		client.WriteMutex.Unlock()
		log.Printf("unlock 2")
		conn.Close()
		removeUsername(username)
		return
	}

	model.ClientsMu.Lock()
	model.Clients[username] = client
	model.ClientsMu.Unlock()
	client.Tank = tank
	// 发送地图配置信息
	SendConfig(client)

	log.Printf("🎮 New connection: %s at (%d,%d) facing %d\n",
		username, tank.LocalX, tank.LocalY, tank.Orientation)

	//printTankShape(tank)

	// 启动客户端消息读取 goroutine
	go readMessages(client)
}

// 处理客户端指令
func readMessages(client *model.Client) {
	// 断开连接后释放资源
	defer func() {
		log.Printf("free resource")
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
	}()

	for {
		// 读取客户端消息
		_, msg, err := client.Conn.ReadMessage()
		if err != nil {
			log.Printf("⚠️ Connection %s error: %v\n", client.ID, err)
			break
		}

		// 解析客户端发送的 JSON 消息
		_, _, payload, err := UnpackWebMessage(msg)
		if err != nil {
			log.Printf("❌ Failed to parse JSON from %s: %v", client.ID, err)
			continue
		}

		// 判断 payload 类型
		if op, ok := payload.(model.OperatePayload); ok {
			// 更新方向
			moveDir := parseDirection(op.Up, op.Down, op.Left, op.Right)

			client.LastActive = time.Now()
			client.Tank.Orientation = moveDir

			if moveDir != model.DirNone {
				client.Tank.GunFacing = moveDir
			}

			// 检查是否开火
			if op.Action == "fire" && client.Tank.Reload == 0 {
				log.Printf("🔥 tank %s fires (reload OK)", client.ID)

				se := OpenFire(client.Tank)

				data, err := RePackWebMessageJson(3, se, "broadcast message gamer")
				if err != nil {
					log.Println("Failed to marshal game state:", err)
					return
				}

				// 广播开火消息
				// 修改开火指令广播中的写入逻辑
				model.ClientsMu.Lock()
				for _, c := range model.Clients {
					log.Printf("lock 3")
					c.WriteMutex.Lock() // 加锁
					err := c.Conn.WriteMessage(websocket.TextMessage, data)
					c.WriteMutex.Unlock()
					log.Printf("unlock 3") // 解锁
					if err != nil {
						log.Printf("Broadcast fire Error sending to %s: %v\n", c.ID, err)
					}
				}
				model.ClientsMu.Unlock()
			}

			//printTankShape(client.Tank)

		} else if oh, ok := payload.(model.HitPayload); ok {
			data, err := RePackWebMessageJson(5, oh, "broadcast message gamer")
			if err != nil {
				log.Println("Failed to marshal game state:", err)
				return
			}
			log.Println("HitPayload")
			model.ClientsMu.Lock()
			for _, c := range model.Clients {
				if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
					log.Printf("Error sending to %s: %v\n", c.ID, err)
				}
			}
			model.ClientsMu.Unlock()

			for _, t := range model.SpawnTanks {
				if t.ID == oh.Victim {
					FreeTank(t)
					removeUsername(t.ID)
					// notice := model.NoticePayload{
					// 	Notice: "you failed",
					// }
					// data, err := RePackWebMessageJson(6, notice, t.ID)
					// if err != nil {
					// 	log.Println("Failed to marshal notice payload:", err)
					// 	return
					// }
					// client.Conn.WriteMessage(websocket.TextMessage, data)
					return
				}
			}
		} else {
			log.Printf("⚠️ payload 不是 OperatePayload，而是：%T", payload)
		}
	}
}

// 广播地图
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
	model.ClientsMu.Lock()
	defer model.ClientsMu.Unlock()
	log.Println(model.Clients)
	for _, c := range model.Clients {
		log.Printf("lock 4")
		c.WriteMutex.Lock() // 加锁
		err := c.Conn.WriteMessage(websocket.TextMessage, data)
		c.WriteMutex.Unlock() // 解锁
		log.Printf("unlock 4")
		if err != nil {
			log.Printf("Broadcast map Error sending to %s: %v\n", c.ID, err)
		}
	}
}

// 链接建立时 发送所需数据
func SendConfig(c *model.Client) {

	config := model.MapConfig{
		Map:          gamemap.GetMap(),
		MapSizeX:     model.MAP_SIZE_X,
		MapSizeY:     model.MAP_SIZE_Y,
		TickInterval: model.TICK_INTERVAL_MS,
		MapRenderMS:  model.MAP_RENDER_MS,
		TankCoordX:   c.Tank.LocalX,
		TankCoordY:   c.Tank.LocalY,
		Tankfacing:   c.Tank.GunFacing,
		ServerID:     c.ID,
	}
	data, err := RePackWebMessageJson(1, config, c.ID)
	if err != nil {
		log.Println("Failed to marshal game state:", err)
		return
	}
	log.Printf("lock 5")
	c.WriteMutex.Lock()
	err = c.Conn.WriteMessage(websocket.TextMessage, data)
	c.WriteMutex.Unlock()
	log.Printf("unlock 5")
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
	//log.Printf("%+v", mes)
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
	//log.Printf("%+v", payload)
	return mes.Type, mes.ID, payload, nil
}

func WaitForCondition(c *model.Client) (bool, string) {
	// log.Println("[WaitForCondition] 开始")

	c.Conn.SetReadDeadline(time.Now().Add(model.WAIT_REPLY_TIME * time.Second))
	msgCh := make(chan []byte)
	timeoutCh := make(chan bool)
	closeCh := make(chan bool)

	// 读取消息的 goroutine
	go func() {
		log.Println("[goroutine] 启动")
		defer log.Println("[goroutine] 退出")

		for {
			// log.Println("[goroutine] 开始 ReadMessage")
			_, msg, err := c.Conn.ReadMessage()
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
		c.Conn.SetReadDeadline(time.Time{})
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
			readNext, username, err := processMessage(c, msg)
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
func processMessage(c *model.Client, msg []byte) (bool, string, error) {
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
	log.Printf("lock 6")
	c.WriteMutex.Lock()
	c.Conn.WriteMessage(websocket.TextMessage, data)
	c.WriteMutex.Unlock()
	log.Printf("lock 6")
	return false, "", nil
}
