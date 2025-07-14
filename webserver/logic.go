package webserver

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	gamemap "example.com/lite_demo/map"
	"example.com/lite_demo/model"
)

var TANK_RELOAD_VALUE = model.TANK_RELOAD_SECONDS * 1000 / model.MAP_RENDER_MS * 5

// 更新游戏状态
func MapRenderloop() {
	ticker := time.NewTicker(model.MAP_RENDER_MS * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// 遍历坦克，把每个活跃的坦克标记到地图上
		// num := runtime.NumGoroutine()
		// fmt.Printf("当前 goroutine 数量：%d\n", num)
		model.SpawnTanksMu.Lock()
		// model.ShotEventsMu.Lock()
		// model.ShotEvents = model.ShotEvents[0:0]
		for _, t := range model.SpawnTanks {
			//坦克移动
			if t.Status == model.StatusTaken {
				gamemap.MarkTankOnMap(t, 0)
				moveTank(t)
				// if t.Trigger { //更新坦克状态时，如果坦克扳机按下则发射子弹
				// 	model.ShotEvents = append(model.ShotEvents, OpenFire(t))
				// }
				gamemap.MarkTankOnMap(t, 1)
			}
			if t.Reload != 0 {
				t.Reload -= 5
			}
		}
		model.SpawnTanksMu.Unlock()
		// model.ShotEventsMu.Unlock()
	}

}

// 开火
func OpenFire(t *model.Tank) *model.ShotEvent {
	var shotevent model.ShotEvent
	shotevent.Facing = t.GunFacing
	shotevent.Tank = t.ID
	shotevent.LocalX = t.LocalX
	shotevent.LocalY = t.LocalY
	t.Reload = uint(TANK_RELOAD_VALUE)
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

// 移动坦克（点逻辑，封装）
func moveTank(t *model.Tank) {
	dx, dy := getDirectionDelta(t.Orientation)
	if t.Orientation == model.DirNone {
		return
	}
	newX := int(t.LocalX) + dx
	newY := int(t.LocalY) + dy

	if !isWithinBounds(newX, newY) {
		//log.Printf("Tank at (%d,%d) cannot move %v — out of bounds", t.LocalX, t.LocalY, t.Orientation)
		return
	}

	if !canMoveTo(newX, newY) {
		//log.Printf("Tank at (%d,%d) cannot move %v — blocked", t.LocalX, t.LocalY, t.Orientation)
		return
	}

	// 更新位置
	t.LocalX = uint(newX)
	t.LocalY = uint(newY)

	//log.Printf("Tank %s moved to (%d,%d) facing %d", t.ID, t.LocalX, t.LocalY, t.Orientation)
}

// 根据方向返回 dx, dy
func getDirectionDelta(dir byte) (int, int) {
	switch dir {
	case model.DirUp:
		return 0, -1
	case model.DirUpRight:
		return 1, -1
	case model.DirRight:
		return 1, 0
	case model.DirDownRight:
		return 1, 1
	case model.DirDown:
		return 0, 1
	case model.DirDownLeft:
		return -1, 1
	case model.DirLeft:
		return -1, 0
	case model.DirUpLeft:
		return -1, -1
	default:
		return 0, 0
	}
}

// 判断新坐标是否在地图内
func isWithinBounds(x, y int) bool {
	return x >= 0 && x < int(model.MAP_SIZE_X) &&
		y >= 0 && y < int(model.MAP_SIZE_Y)
}

// 判断目标位置是否可以移动（不被占用）
func canMoveTo(x, y int) bool {
	return model.Map[y][x] == 0
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
func allocateTank(id string) *model.Tank {
	model.SpawnTanksMu.Lock()
	defer model.SpawnTanksMu.Unlock()
	for {
		r_x := rand.Intn(int(model.MAP_SIZE_X))
		r_y := rand.Intn(int(model.MAP_SIZE_Y))
		if model.Map[r_y][r_x] == 0 {
			t := model.Tank{
				LocalX:      uint(r_x),
				LocalY:      uint(r_y),
				Reload:      0,
				Trigger:     false,
				GunFacing:   model.DirDown,
				Status:      model.StatusTaken,
				Orientation: model.DirNone,
				ID:          id,
			}
			model.SpawnTanks = append(model.SpawnTanks, &t)
			gamemap.MarkTankOnMap(&t, 1)
			return &t
		}
	}
}

// 释放出生点
func FreeTank(target *model.Tank) {
	model.SpawnTanksMu.Lock()
	defer model.SpawnTanksMu.Unlock()
	gamemap.MarkTankOnMap(target, 0)
	for i, t := range model.SpawnTanks {
		if t == target {
			// 用最后一个覆盖自己
			model.SpawnTanks[i] = model.SpawnTanks[len(model.SpawnTanks)-1]
			model.SpawnTanks = model.SpawnTanks[:len(model.SpawnTanks)-1]
			return
		}
	}
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
