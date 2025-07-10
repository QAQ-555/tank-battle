package gamemap

import (
	"log"

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
