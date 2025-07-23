package webserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"example.com/lite_demo/model"
	msgpack "example.com/lite_demo/test"
	datastruct "example.com/lite_demo/test/datastruct"
	"example.com/lite_demo/test/payload"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// 处理链接请求
func Handler(w http.ResponseWriter, r *http.Request) {
	// 1. 升级 HTTP 连接为 WebSocket
	conn, err := model.UP.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	// 2. 创建客户端实例
	client := &model.Client{
		Conn:       conn,
		LastActive: time.Now(),
	}

	// 3. 发送连接成功通知
	if err := sendConnectNotice(client); err != nil {
		log.Println("Failed to marshal notice payload:", err)
		return
	}
	debug := true
	username := ""
	//4. 等待客户端提交用户名
	if debug {
		username = uuid.NewString()
	} else {
		ok, username := waitForUsername(client)
		if !ok {
			log.Println("⏳ 超时或失败，未获取 username")
			conn.Close() // 关闭连接，释放资源
			return
		}
		log.Println("✅ 成功获取 username:", username)
		// username := uuid.New().String()
		client.ID = username
	}

	log.Println("✅ 成功获取 username:", username)
	// username := uuid.New().String()
	client.ID = username
	model.ClientsMu.Lock()
	model.Clients[username] = client
	model.ClientsMu.Unlock()
	// 5. 为客户端分配坦克
	tank := allocateTank(username)
	if tank == nil {
		log.Printf("❌ No available spawn point for %s\n", username)
		sendNoSpawnNotice(client, username)
		conn.Close()
		removeUsername(username)
		return
	}

	// 6. 添加客户端到全局列表

	client.Tank = tank

	// 7. 发送配置信息
	SendConfig(client)

	log.Printf("🎮 New connection: %s at (%d,%d) facing %d\n",
		username, tank.LocalX, tank.LocalY, tank.Orientation)

	// 8. 启动消息读取 goroutine
	go handleClientMessages(client)
}

// 发送连接成功通知
func sendConnectNotice(client *model.Client) error {
	msg := &msgpack.MsgPack{
		Type:   []byte{model.TypeMessage},
		Target: client.ID,
		Payload: &msgpack.MsgPack_Notice{
			Notice: "connect success",
		},
	}
	msg_b, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	client.WriteMutex.Lock()
	defer client.WriteMutex.Unlock()
	err = client.Conn.WriteMessage(websocket.BinaryMessage, msg_b)
	return err
}

// 发送无可用出生点通知
func sendNoSpawnNotice(client *model.Client, username string) {
	data, err := RePackWebMessageJson(4, []byte("No available spawn point"), username)
	if err != nil {
		log.Println("Failed to marshal game state:", err)
		return
	}

	client.WriteMutex.Lock()
	client.Conn.WriteMessage(websocket.TextMessage, data)
	client.WriteMutex.Unlock()

}

// 处理客户端消息循环
func handleClientMessages(client *model.Client) {
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
		protoMsg := &msgpack.MsgPack{}
		err = proto.Unmarshal(msg, protoMsg)
		// 打印调试信息
		log.Printf("Received message: %+v\n", protoMsg)

		if err == nil {
			if protoMsg.GetType() == nil {
				log.Printf("⚠️ Connection %s message is nil/err\n", client.ID)
				continue
			}
			switch protoMsg.Type[0] {
			case model.TypeEmojiS:
				// 处理表情
				if protoMsg.GetEmoji() == nil {
					log.Printf("⚠️ TypeEmojiS Connection %s message is nil/err\n", client.ID)
					continue
				}
				processEmojiPayload(protoMsg)
			case model.TypeHitNotice:
				// 处理击中通知
				if protoMsg.GetHit() == nil {
					log.Printf("⚠️ TypeHitNotice Connection %s message is nil/err\n", client.ID)
					continue
				}
				hp := model.HitPayload{
					Username: protoMsg.GetHit().Username,
					Victim:   protoMsg.GetHit().Victim,
				}
				processHitPayload(hp)
			case model.TypeTankOperation:
				// 处理坦克操作指令
				if protoMsg.GetOperate() == nil {
					log.Printf("⚠️ TypeTankOperation Connection %s message is nil/err\n", client.ID)
					continue
				}
				op := model.OperatePayload{
					Up:     protoMsg.GetOperate().Up,
					Down:   protoMsg.GetOperate().Down,
					Left:   protoMsg.GetOperate().Left,
					Right:  protoMsg.GetOperate().Right,
					Action: protoMsg.GetOperate().Action,
				}
				processOperatePayload(client, op)
			case model.TypeRespawnRequest:
				if protoMsg.GetRequest() == nil {
					log.Printf("⚠️  TypeRespawnRequest Connection %s message is nil/err\n", client.ID)
					continue
				}
				rp := model.RespawnPayload{
					Username: protoMsg.GetRequest().Username,
					Success:  protoMsg.GetRequest().Success,
				}
				processRespawnPayload(rp)
			default:
				log.Printf("⚠️ unknow type %s : %+v\n", protoMsg.Type, protoMsg.GetPayload())
			}
		} else {
			log.Printf("⚠️ not proto message : %v\n", err)
			// 解析客户端发送的 JSON 消消息
			_, _, payload, err := UnpackWebMessage(msg)
			if err != nil {
				log.Printf("❌ Failed to parse JSON from %s: %v", client.ID, err)
				continue
			}

			switch v := payload.(type) {
			case model.OperatePayload:
				processOperatePayload(client, v)
			case model.HitPayload:
				processHitPayload(v)
			case model.RespawnPayload:
				processRespawnPayload(v)
			default:
				log.Printf("⚠️ payload 不是 OperatePayload/HitPayload，而是：%T", payload)
			}
		}
	}
}

// 处理坦克操作指令
func processOperatePayload(client *model.Client, op model.OperatePayload) {
	log.Printf("processOperatePayload %+v", op)
	moveDir := parseDirection(op.Up, op.Down, op.Left, op.Right)
	client.LastActive = time.Now()
	client.Tank.Orientation = moveDir

	if moveDir != model.DirNone {
		client.Tank.GunFacing = moveDir
	}
	log.Printf(ColorGreen+"[move event]"+ColorReset+" tank %s move to (%d,%d) facing %d",
		client.ID, client.Tank.LocalX, client.Tank.LocalY, client.Tank.Orientation)
	// 检查是否开火
	if op.Action == "fire" && client.Tank.Reload == 0 {
		se := OpenFire(client.Tank)
		log.Printf(ColorBlue+"[shot event]"+ColorReset+" tank %s fires (reload OK)", client.ID)

		seProto := &msgpack.MsgPack{
			Type:   []byte{model.TypeShotEvent},
			Target: "all",
			Payload: &msgpack.MsgPack_Shot{
				Shot: &payload.ShotPayload{
					Username: client.ID,
					X:        uint32(se.LocalX),
					Y:        uint32(se.LocalY),
					Facing:   []byte{byte(se.Facing)},
				},
			},
		}

		data, err := proto.Marshal(seProto)
		if err != nil {
			log.Println("Failed to marshal game state:", err)
			return
		}
		broadcastToAllClients(data, "Broadcast fire")
	}
}

// 处理命中事件
func processHitPayload(oh model.HitPayload) {
	model.ClientsMu.Lock()

	var victimClient *model.Client
	for _, c := range model.Clients {
		if c.Tank != nil && c.Tank.ID == oh.Victim {
			victimClient = c
			break
		} else if c.Tank != nil && c.Tank.ID == oh.Username {
			c.Tank.Point += 1
		}
	}
	model.ClientsMu.Unlock()
	if victimClient == nil {
		log.Printf("⚠️ Victim tank %s not found among clients, hit by %s", oh.Victim, oh.Username)
		return
	}

	// 释放并重新分配坦克

	victimClient.Tank.Status = model.StatusFree
	msg := &msgpack.MsgPack{
		Type:   []byte{model.TypeTankChangeEvent},
		Target: "all",
		Payload: &msgpack.MsgPack_TankChange{
			TankChange: &payload.TankChangePayload{
				Username: victimClient.ID,
				TurnTo:   false,
				X:        uint32(victimClient.Tank.LocalX),
				Y:        uint32(victimClient.Tank.LocalY),
			},
		},
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		log.Println("Failed to marshal game state:", err)
	}

	broadcastToAllClients(data, "Broadcast change")

	// // 只有存在被击中人时才广播
	// data, err = RePackWebMessageJson(7, oh, "broadcast message gamer")
	// if err != nil {
	// 	log.Println("Failed to marshal game state:", err)
	// 	return
	// }

	log.Printf(ColorRed+"[hit event]"+ColorReset+" tank %s hit by %s", oh.Victim, oh.Username)
	// broadcastToAllClients(data, "Broadcast victim")
}

func processRespawnPayload(p model.RespawnPayload) {
	log.Printf("[respawn event] 开始处理用户 %s 请求重生", p.Username)

	model.ClientsMu.Lock()
	log.Printf("[respawn event] 已获取 ClientsMu 锁")

	var targetClient *model.Client
	for _, c := range model.Clients {
		if c.Tank != nil && c.Tank.ID == p.Username {
			targetClient = c
			break
		}
	}

	// 找到目标客户端后，释放 ClientsMu 锁
	model.ClientsMu.Unlock()
	log.Printf("[respawn event] 释放 ClientsMu 锁")

	if targetClient == nil {
		log.Printf("[respawn event] 未找到用户 %s", p.Username)
		return
	}

	// 调用 allocateTank 函数，此时无锁冲突
	newTank := allocateTank(p.Username)
	if newTank == nil {
		log.Printf("[respawn event] allocateTank 失败，无法为 %s 分配新坦克", p.Username)
		return
	}

	// 重新获取 ClientsMu 锁以更新客户端信息
	model.ClientsMu.Lock()
	log.Printf("[respawn event] 重新获取 ClientsMu 锁")
	defer func() {
		log.Printf("[respawn event] 释放 ClientsMu 锁")
		model.ClientsMu.Unlock()
	}()

	// 确保目标客户端仍然有效
	if targetClient.Tank != nil && targetClient.Tank.ID == p.Username {
		newTank.Point = targetClient.Tank.Point
		FreeTank(targetClient.Tank)
		targetClient.Tank = newTank
	} else {
		log.Printf("[respawn event] 处理过程中用户 %s 已断开连接", p.Username)
	}

}

// 广播消息到所有客户端
func broadcastToAllClients(data []byte, logPrefix string) {
	model.ClientsMu.Lock()
	defer model.ClientsMu.Unlock()
	for _, c := range model.Clients {

		c.WriteMutex.Lock()
		err := c.Conn.WriteMessage(websocket.BinaryMessage, data)
		c.WriteMutex.Unlock()

		if err != nil {
			log.Printf("%s Error sending to %s: %v\n", logPrefix, c.ID, err)
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
	data, err := proto.Marshal(state)
	if err != nil {
		log.Println("Failed to marshal game state:", err)
		return
	}
	model.ClientsMu.Lock()
	defer model.ClientsMu.Unlock()
	for _, c := range model.Clients {

		c.WriteMutex.Lock()
		err := c.Conn.WriteMessage(websocket.BinaryMessage, data)
		c.WriteMutex.Unlock()

		if err != nil {
			log.Printf("Broadcast map Error sending to %s: %v\n", c.ID, err)
		}
	}
}

// 链接建立时 发送所需数据
func SendConfig(c *model.Client) {
	configProto := msgpack.MsgPack{
		Type:   []byte{model.TypeInitData},
		Target: "connect machine",
		Payload: &msgpack.MsgPack_GameConfig{
			GameConfig: &payload.GameConfigPayload{
				Tanks: []*datastruct.Tank{},
				Map: &payload.Blockspayload{
					Blockpoints: []*datastruct.Block{},
				},
			},
		},
	}
	configProto.Payload.(*msgpack.MsgPack_GameConfig).GameConfig.MapSizeX = uint32(model.MAP_SIZE_X)
	configProto.Payload.(*msgpack.MsgPack_GameConfig).GameConfig.MapSizeY = uint32(model.MAP_SIZE_Y)
	configProto.Payload.(*msgpack.MsgPack_GameConfig).GameConfig.TickInterval = model.TICK_INTERVAL_MS
	configProto.Payload.(*msgpack.MsgPack_GameConfig).GameConfig.MapRenderMS = model.MAP_RENDER_MS
	mtanks := GetActiveTanks()
	// 初始化 Payload 为 *msgpack.MsgPack_GameStatus 类型
	for _, val := range mtanks {
		tmp := &datastruct.Tank{
			X:           uint32(val.LocalX),
			Y:           uint32(val.LocalY),
			Reload:      uint32(val.Reload),
			Trigger:     val.Trigger,
			GunFacing:   []byte{val.GunFacing},
			Status:      []byte{val.Status},
			Orientation: []byte{val.Orientation},
			Username:    val.ID,
			Point:       int32(val.Point),
		}
		configProto.Payload.(*msgpack.MsgPack_GameConfig).GameConfig.Tanks = append(configProto.Payload.(*msgpack.MsgPack_GameConfig).GameConfig.Tanks, tmp)
	}

	for _, val := range model.Blocks {
		tmp := &datastruct.Block{
			P:   []*datastruct.Mappoint{},
			L:   int32(val.L),
			W:   int32(val.W),
			Dir: []byte{val.Dir},
		}
		for _, p := range val.P {
			tmp.P = append(tmp.P, &datastruct.Mappoint{
				X: int32(p.X),
				Y: int32(p.Y),
			})
		}
		configProto.Payload.(*msgpack.MsgPack_GameConfig).GameConfig.Map.Blockpoints = append(configProto.Payload.(*msgpack.MsgPack_GameConfig).GameConfig.Map.Blockpoints, tmp)
	}
	data, err := proto.Marshal(&configProto)
	if err != nil {
		log.Println("Failed to marshal game state:", err)
		return
	}

	c.WriteMutex.Lock()
	err = c.Conn.WriteMessage(websocket.BinaryMessage, data)
	c.WriteMutex.Unlock()

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
	case 17:
		var hp model.HitPayload
		if err := json.Unmarshal(payloadBytes, &hp); err != nil {
			return 0, "", nil, err
		}
		payload = hp
	case 18:
		var rp model.RespawnPayload
		if err := json.Unmarshal(payloadBytes, &rp); err != nil {
			return 0, "", nil, err
		}
		payload = rp
	default:
		return 0, "", nil, fmt.Errorf("unknown message type: %d", mes.Type)
	}
	//log.Printf("%+v", payload)
	return mes.Type, mes.ID, payload, nil
}

// 等待客户端注册用户名
func waitForUsername(c *model.Client) (bool, string) {
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
			log.Printf("waitForUsername")
			readNext, username, err := handleRegisterMessage(c, msg)
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

// 处理注册消息
func handleRegisterMessage(c *model.Client, msg []byte) (bool, string, error) {
	protoMsg := &msgpack.MsgPack{}
	err := proto.Unmarshal(msg, protoMsg)
	if err == nil {
		log.Printf("prorobuf side %+v", protoMsg)

		log.Printf("%d", protoMsg.GetType()[0])
		if protoMsg.GetType()[0] != model.TypeRegisterRequest {
			return false, "", nil
		}
		if protoMsg.GetRequest() == nil {
			return false, "", nil
		}

		if protoMsg.GetRequest().Success && isUsernameLegal(protoMsg.GetRequest().Username) {
			log.Printf("username is legal")
			model.UsernameMu.Lock()
			model.Usernames = append(model.Usernames, protoMsg.GetRequest().Username)
			model.UsernameMu.Unlock()
			return true, protoMsg.GetRequest().Username, nil
		}
		log.Printf("username is empty or already exists")
		msgpr := &msgpack.MsgPack{
			Type:   []byte{model.TypeErrorNotice},
			Target: c.ID,
			Payload: &msgpack.MsgPack_Notice{
				Notice: "username is empty or already exists",
			},
		}

		data, err := proto.Marshal(msgpr)
		if err != nil {
			log.Println("Failed to marshal notice payload:", err)
			return false, "", nil
		}

		c.WriteMutex.Lock()
		c.Conn.WriteMessage(websocket.TextMessage, data)
		c.WriteMutex.Unlock()

		return false, "", nil
	} else {
		log.Printf("json side")
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

		c.WriteMutex.Lock()
		c.Conn.WriteMessage(websocket.TextMessage, data)
		c.WriteMutex.Unlock()

		return false, "", nil
	}
}

func processEmojiPayload(emoji *msgpack.MsgPack) {
	log.Printf("✅ Connection	: %+v\n", emoji.GetEmoji())
	emoji.Target = "board cast back"
	data, err := proto.Marshal(emoji)
	if err != nil {
		log.Println("Failed to marshal emoji payload:", err)
		return
	}
	broadcastToAllClients(data, "all")
}
