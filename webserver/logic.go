package webserver

import (
	"fmt"
	"log"
	"time"

	gamemap "example.com/lite_demo/map"
	"example.com/lite_demo/model"
)

// 更新游戏状态
func MapRenderloop() {
	ticker := time.NewTicker(model.MAP_RENDER_MS * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// 遍历坦克，把每个活跃的坦克标记到地图上

		model.SpawnTanksMu.Lock()
		model.ShotEventsMu.Lock()
		for _, t := range model.SpawnTanks {
			//坦克移动
			if t.Status == model.StatusTaken {
				gamemap.MarkTankOnMap(t, 0)
				moveTank(t)
				if t.Trigger { //更新坦克状态时，如果坦克扳机按下则发射子弹
					model.ShotEvents = append(model.ShotEvents, OpenFire(t))
				}
			}
			//发射子弹
		}
		// for _, b := range activeBullets {

		// }
		model.SpawnTanksMu.Unlock()
		model.ShotEventsMu.Unlock()
		model.FlagChan <- true
	}

}

// 开火
func OpenFire(t *model.Tank) *model.ShotEvent {
	var shotevent model.ShotEvent
	shotevent.Facing = t.GunFacing
	shotevent.Tank = t.ID
	switch t.GunFacing {
	case model.DirDown:
		shotevent.LocalX = t.LocalX
		shotevent.LocalY = t.LocalY + 2
	case model.DirUp:
		shotevent.LocalX = t.LocalX
		shotevent.LocalY = t.LocalY - 2
	case model.DirLeft:
		shotevent.LocalX = t.LocalX - 2
		shotevent.LocalY = t.LocalY
	case model.DirRight:
		shotevent.LocalX = t.LocalX + 2
		shotevent.LocalY = t.LocalY
	case model.DirUpLeft:
		shotevent.LocalX = t.LocalX - 2
		shotevent.LocalY = t.LocalY - 2
	case model.DirUpRight:
		shotevent.LocalX = t.LocalX + 2
		shotevent.LocalY = t.LocalY - 2
	case model.DirDownLeft:
		shotevent.LocalX = t.LocalX - 2
		shotevent.LocalY = t.LocalY + 2
	case model.DirDownRight:
		shotevent.LocalX = t.LocalX + 2
		shotevent.LocalY = t.LocalY + 2
	default:
		// 如果方向未知，就放在坦克正中央
		shotevent.LocalX = t.LocalX
		shotevent.LocalY = t.LocalY
	}
	t.Reload = 500
	t.Trigger = false
	printTankShape(t)
	log.Printf("shoting shotevent=%+v\n", shotevent)
	return &shotevent
}

// 打印坦克
func printTankShape(t *model.Tank) {
	dirSymbols := map[byte]string{
		model.DirUp:        "↑",
		model.DirUpRight:   "↗",
		model.DirRight:     "→",
		model.DirDownRight: "↘",
		model.DirDown:      "↓",
		model.DirDownLeft:  "↙",
		model.DirLeft:      "←",
		model.DirUpLeft:    "↖",
		model.DirNone:      "o",
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
		return model.DirUpLeft
	case up && right && !down && !left:
		return model.DirUpRight
	case down && left && !up && !right:
		return model.DirDownLeft
	case down && right && !up && !left:
		return model.DirDownRight
	case up && !down:
		return model.DirUp
	case down && !up:
		return model.DirDown
	case left && !right:
		return model.DirLeft
	case right && !left:
		return model.DirRight
	default:
		return model.DirNone
	}
}

// 移动坦克
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

// 获取所有活动中坦克
func GetActiveTanks() []*model.Tank {
	model.SpawnTanksMu.Lock()
	defer model.SpawnTanksMu.Unlock()

	var active []*model.Tank
	for _, t := range model.SpawnTanks {
		if t.Status != model.StatusFree {
			active = append(active, t)
		}
	}
	return active
}

// 构建游戏状态结构体
func BuildGameState() *model.GameState {
	return &model.GameState{
		Tanks:      GetActiveTanks(),
		ShotEvents: model.ShotEvents,
		// Items: GetActiveItems(),
		// Map: GetMap(),
	}
}

// 初始化出生点
func InitSpawnTanks() {

	coords := [][2]uint{
		{1, 1},                    // 上左
		{model.MAP_SIZE_X / 2, 1}, // 上中
		{model.MAP_SIZE_X - 2, 1}, // 上右
		{model.MAP_SIZE_X - 2, model.MAP_SIZE_Y / 2}, // 右中
		{model.MAP_SIZE_X - 2, model.MAP_SIZE_Y - 2}, // 下右
		{model.MAP_SIZE_X / 2, model.MAP_SIZE_Y - 2}, // 下中
		{1, model.MAP_SIZE_Y - 2},                    // 下左
		{1, model.MAP_SIZE_Y / 2},                    // 左中
	}

	for _, c := range coords {
		x, y := c[0], c[1]
		tank := &model.Tank{
			LocalX:      x,
			LocalY:      y,
			Reload:      0,
			Trigger:     false,
			Status:      model.StatusFree,
			GunFacing:   model.DirDown,
			Orientation: model.DirNone,
		}
		model.SpawnTanks = append(model.SpawnTanks, tank)
	}
}

// 分配出生点
func allocateTank() *model.Tank {
	model.SpawnTanksMu.Lock()
	defer model.SpawnTanksMu.Unlock()

	for _, t := range model.SpawnTanks {
		if t.Status == model.StatusFree {
			t.Status = model.StatusTaken
			gamemap.MarkTankOnMap(t, 1)
			return t
		}
	}
	return nil
}

// 释放出生点
func freeTank(t *model.Tank) {
	model.SpawnTanksMu.Lock()
	defer model.SpawnTanksMu.Unlock()

	t.Status = model.StatusFree
	gamemap.MarkTankOnMap(t, 0)
}

// 从 model.Usernames 切片里删除 username
func removeUsername(username string) {
	model.UsernameMu.Lock()
	defer model.UsernameMu.Unlock()

	newList := make([]string, 0, len(model.Usernames))
	for _, u := range model.Usernames {
		if u != username {
			newList = append(newList, u)
		}
	}
	model.Usernames = newList

	log.Printf("已删除用户名: %s", username)
}

func isUsernameLegal(username string) bool {
	// 先判断 username 是否为空
	if username == "" {
		return false
	}

	// 遍历现有的 model.Usernames
	for _, u := range model.Usernames {
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
