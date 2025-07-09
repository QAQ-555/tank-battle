package gamemap

import (
	"log"
	"time"

	"example.com/lite_demo/model"
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
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			x := int(t.LocalX) + dx
			y := int(t.LocalY) + dy
			if x >= 0 && x < int(model.MAP_SIZE_X) && y >= 0 && y < int(model.MAP_SIZE_Y) {
				model.Map[y][x] = val
			}
		}
	}
}

// 更新游戏状态
func MapRenderloop() {
	ticker := time.NewTicker(model.MAP_RENDER_MS * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// 遍历坦克，把每个活跃的坦克标记到地图上

		model.SpawnTanksMu.Lock()
		model.ActiveBulletsMu.Lock()
		for _, t := range model.SpawnTanks {
			//坦克移动
			if t.Status == model.StatusTaken {
				MarkTankOnMap(t, 0)
				moveTank(t)
				// if t.Trigger { //更新坦克状态时，如果坦克扳机按下则发射子弹
				// 	activeBullets = append(activeBullets, openFire(t))
				// }
			}
			//发射子弹
		}
		// for _, b := range activeBullets {

		// }
		model.SpawnTanksMu.Unlock()
		model.ActiveBulletsMu.Unlock()
		model.FlagChan <- true
	}

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
