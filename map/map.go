package gamemap

import (
	"log"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"example.com/lite_demo/model"
	"github.com/fogleman/gg"
	"github.com/fogleman/poissondisc"
	"github.com/gorilla/websocket"
)

func FindBlock(x, y, tx, ty uint, dir byte) bool {
	// 获取方向的偏移量
	dx, dy := getDirectionDelta(dir)
	//log.Printf("FindBlock: x=%d, y=%d, tx=%d, ty=%d, dx = %d, dy = %d\n", x, y, tx, ty, dx, dy)
	currentX, currentY := int(x), int(y)
	for {
		//log.Printf("currentX=%d, currentY=%d", currentX, currentY)
		currentX += dx
		currentY += dy
		//log.Printf("nextX=%d, nextY=%d", currentX, currentY)
		// 检查是否超出地图范围
		if !isWithinBounds(currentX, currentY) {
			log.Printf("FindBlock: out of bounds(%d,%d)", currentX, currentY)
			return false
		}

		// 检查是否到达目标点
		if uint(currentX) == tx && uint(currentY) == ty {
			break
		}

		// 检查是否有阻挡
		if model.Map[currentY][currentX] != 0 {
			log.Printf("FindBlock: hit block(%d,%d)", currentX, currentY)
			return false
		}
	}

	return true
}

func isWithinBounds(x, y int) bool {
	return x >= 0 && x < int(model.MAP_SIZE_X) &&
		y >= 0 && y < int(model.MAP_SIZE_Y)
}

// 清空地图
func clearMap() {
	for y := 0; y < int(model.MAP_SIZE_Y); y++ {
		for x := 0; x < int(model.MAP_SIZE_X); x++ {
			model.Map[y][x] = 0
		}
	}
}

var allowedDirsMap = map[byte][]byte{
	// 上方向: 本身 → 右上/左上(相邻1个) → 右/左(相邻2个)
	model.DirUp: {model.DirUp, model.DirUpRight, model.DirUpLeft, model.DirRight, model.DirLeft},
	// 下方向: 本身 → 右下/左下(相邻1个) → 右/左(相邻2个)
	model.DirDown: {model.DirDown, model.DirDownRight, model.DirDownLeft, model.DirRight, model.DirLeft},
	// 左方向: 本身 → 左上/左下(相邻1个) → 上/下(相邻2个)
	model.DirLeft: {model.DirLeft, model.DirUpLeft, model.DirDownLeft, model.DirUp, model.DirDown},
	// 右方向: 本身 → 右上/右下(相邻1个) → 上/下(相邻2个)
	model.DirRight: {model.DirRight, model.DirUpRight, model.DirDownRight, model.DirUp, model.DirDown},
	// 右上方向: 本身 → 上/右(相邻1个) → 左上/右下(相邻2个)
	model.DirUpRight: {model.DirUpRight, model.DirUp, model.DirRight, model.DirUpLeft, model.DirDownRight},
	// 左上方向: 本身 → 上/左(相邻1个) → 右上/左下(相邻2个)
	model.DirUpLeft: {model.DirUpLeft, model.DirUp, model.DirLeft, model.DirUpRight, model.DirDownLeft},
	// 右下方向: 本身 → 下/右(相邻1个) → 右上/左下(相邻2个)
	model.DirDownRight: {model.DirDownRight, model.DirDown, model.DirRight, model.DirUpRight, model.DirDownLeft},
	// 左下方向: 本身 → 下/左(相邻1个) → 左上/右下(相邻2个)
	model.DirDownLeft: {model.DirDownLeft, model.DirDown, model.DirLeft, model.DirUpLeft, model.DirDownRight},
}

func GenerateRiver(x, y int, steps int) int {
	// 初始化起点
	var bulidedpoints []model.MapPoint
	bulidedpoints = append(bulidedpoints, model.MapPoint{X: uint(x), Y: uint(y)})
	//log.Printf("[河流生成] 开始生成河流 - 起点: (%d,%d), 计划步数: %d\n", x, y, steps)

	model.Map[y][x] = 2

	// 初始化方向偏好（随机初始方向）
	dirOptions := []byte{1, 2, 3, 4, 6, 7, 8, 9}
	preferredDir := dirOptions[rand.Intn(len(dirOptions))]
	allowedDirsOptions := allowedDirsMap[preferredDir]
	//log.Printf("%v", allowedDirsOptions)
	//log.Printf("[河流生成] 初始方向偏好: %d", preferredDir)
	weights := []float64{8, 5, 5}
	for i := 0; i < steps; i++ {
		if i%15 == 0 {
			preferredDir = allowedDirsOptions[randomdir(weights)]
			allowedDirsOptions = allowedDirsMap[preferredDir]
			//log.Printf("[河流生成] 更新方向偏好: %d", preferredDir)

		}
		newX, newY := getDirectionDelta(preferredDir)
		//log.Printf("%v", preferredDir)
		if newX+x < 0 || newY+y < 0 || newX+x >= int(model.MAP_SIZE_X) || newY+y >= int(model.MAP_SIZE_Y) {
			if i < steps/2 {
				//log.Printf("[河流生成] %d 超出地图范围，跳过", i)
				for _, p := range bulidedpoints {
					model.Map[p.Y][p.X] = 0
				}
			}
			break
		}

		if model.Map[y+newY][x+newX] != 0 {
			if i < steps/2 {
				//log.Printf("[河流生成] %d 已经有点了，跳过", i)
				for _, p := range bulidedpoints {
					model.Map[p.Y][p.X] = 0
				}
			}
			break
		}
		//log.Printf("per (%d,%d) next(%d,%d) step(%d,%d),%d,%d", x, y, newX+x, newY+y, newX, newY, preferredDir, i)
		model.Map[y+newY][x+newX] = 2
		bulidedpoints = append(bulidedpoints, model.MapPoint{X: uint(x + newX), Y: uint(y + newY)})
		x = x + newX
		y = y + newY
		//log.Printf("[河流生成] 生成新点: (%d,%d)", x, y)
	}
	grownCount := 0

	return grownCount
}

func GenerateTree(x, y int, steps int) int {
	// 定义四个方向（上、下、左、右）
	directions := [4]struct{ X, Y int }{
		{1, 0},  // 右
		{-1, 0}, // 左
		{0, 1},  // 下
		{0, -1}, // 上
	}

	// BFS 队列，存储待处理的点
	queue := []model.MapPoint{{X: uint(x), Y: uint(y)}}
	// 记录已访问的点
	visited := make(map[model.MapPoint]bool)
	visited[model.MapPoint{X: uint(x), Y: uint(y)}] = true
	// 记录生成的河流点
	builtPoints := []model.MapPoint{{X: uint(x), Y: uint(y)}}
	// 标记起点为河流（假设 3 代表河流）
	model.Map[y][x] = 3

	// BFS 遍历，限定步数
	for step := 0; step < steps && len(queue) > 0; step++ {
		currentLevelSize := len(queue)
		for i := 0; i < currentLevelSize; i++ {
			current := queue[0]
			queue = queue[1:]

			for _, dir := range directions {
				newX := int(current.X) + dir.X
				newY := int(current.Y) + dir.Y

				if newX < 0 || newY < 0 || newX >= int(model.MAP_SIZE_X) || newY >= int(model.MAP_SIZE_Y) {
					continue
				}
				if model.Map[newY][newX] == 2 {
					return 0
				}
				newPoint := model.MapPoint{X: uint(newX), Y: uint(newY)}

				if visited[newPoint] || model.Map[newY][newX] != 0 {
					continue
				}

				model.Map[newY][newX] = 3
				builtPoints = append(builtPoints, newPoint)
				visited[newPoint] = true
				queue = append(queue, newPoint)
			}
		}
	}

	return 0
}

func GenerateCircle(centerX, centerY, radius int) int {
	directions := [4]struct{ X, Y int }{
		{1, 0}, {-1, 0}, {0, 1}, {0, -1},
	}

	queue := []model.MapPoint{{X: uint(centerX), Y: uint(centerY)}}
	visited := make(map[model.MapPoint]bool)
	visited[model.MapPoint{X: uint(centerX), Y: uint(centerY)}] = true
	model.Map[centerY][centerX] = 3

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, dir := range directions {
			newX := int(current.X) + dir.X
			newY := int(current.Y) + dir.Y

			if newX < 0 || newY < 0 || newX >= int(model.MAP_SIZE_X) || newY >= int(model.MAP_SIZE_Y) {
				continue
			}

			if model.Map[newY][newX] == 2 {
				return 0
			}

			newPoint := model.MapPoint{X: uint(newX), Y: uint(newY)}
			if visited[newPoint] || model.Map[newY][newX] != 0 {
				continue
			}

			dx, dy := newX-centerX, newY-centerY
			if dx*dx+dy*dy <= radius*radius {
				model.Map[newY][newX] = 3

				visited[newPoint] = true
				queue = append(queue, newPoint)
			}
		}
	}
	return 0
}

func CheckZeroConnectivity() bool {
	// 找到第一个 0 作为起点
	var startX, startY int
	found := false
	for y := 0; y < int(model.MAP_SIZE_Y); y++ {
		for x := 0; x < int(model.MAP_SIZE_X); x++ {
			if model.Map[y][x] == 0 {
				startX, startY = x, y
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return true // 没有 0，无需检查
	}

	// Flood Fill 检查连通性
	directions := [4]struct{ X, Y int }{
		{1, 0}, {-1, 0}, {0, 1}, {0, -1},
	}
	visited := make([][]bool, model.MAP_SIZE_Y)
	for i := range visited {
		visited[i] = make([]bool, model.MAP_SIZE_X)
	}
	queue := []struct{ X, Y int }{{startX, startY}}
	visited[startY][startX] = true
	zeroCount := 1 // 起点是一个 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, dir := range directions {
			newX, newY := current.X+dir.X, current.Y+dir.Y
			if newX >= 0 && newY >= 0 && newX < int(model.MAP_SIZE_X) && newY < int(model.MAP_SIZE_Y) {
				if !visited[newY][newX] && model.Map[newY][newX] == 0 {
					visited[newY][newX] = true
					queue = append(queue, struct{ X, Y int }{newX, newY})
					zeroCount++
				}
			}
		}
	}

	// 统计总共有多少个 0
	totalZeros := 0
	for y := 0; y < int(model.MAP_SIZE_Y); y++ {
		for x := 0; x < int(model.MAP_SIZE_X); x++ {
			if model.Map[y][x] == 0 {
				totalZeros++
			}
		}
	}

	// 如果 Flood Fill 访问的 0 数量等于总数，说明全部连通
	return zeroCount == totalZeros
}

func randomdir(w []float64) int {

	total_weight := 0.0
	for i := 0; i < len(w); i++ {
		total_weight += w[i]
	}
	r := rand.Float64() * total_weight
	for i, w := range w {
		r -= w
		if r < 0 {
			return i
		}
	}
	return 0 // 默认返回第一个方向
}

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
	// 		x := int(t.LocalX) + dxX
	// 		y := int(t.LocalY) + dy
	// 		if x >= 0 && x < int(model.MAP_SIZE_X) && y >= 0 && y < int(model.MAP_SIZE_Y) {
	model.Map[t.LocalY][t.LocalX] = val
	// 		}
	// 	}
	// }
}

func getMapAsString() string {
	var sb strings.Builder
	for y := 0; y < int(model.MAP_SIZE_Y); y++ {
		for x := 0; x < int(model.MAP_SIZE_X); x++ {
			cell := model.Map[y][x]
			if cell == 0 {
				sb.WriteRune('□') // 空地
			} else {
				sb.WriteRune('■') // 占用
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func WsMapHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := model.UP.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(time.Millisecond * model.TICK_INTERVAL_MS)
	defer ticker.Stop()

	for range ticker.C {
		mapStr := getMapAsString()
		err := conn.WriteMessage(websocket.TextMessage, []byte(mapStr))
		if err != nil {
			log.Println("write map error:", err)
			return
		}
	}
}

func Trywall(r_x, r_y int, dir byte) bool {
	//log.Printf("try build wall at (%d,%d), dir: %d", r_x, r_y, dir)
	// 初始化标志为 true
	flag := true

	if dir != 2 && dir != 4 && dir != 6 && dir != 8 {
		return false
	}

	if dir == 6 || dir == 4 {
		// 水平方向检测
		// 上水平边界检测
		flag = flag && FindBlock(uint(r_x+int(model.BOCLK_LENTH/2)), uint(r_y+model.BLOCK_WIDTH/2), uint(r_x-int(model.BOCLK_LENTH/2)), uint(r_y+model.BLOCK_WIDTH/2), model.DirLeft)
		// 下水平边界检测
		flag = flag && FindBlock(uint(r_x+int(model.BOCLK_LENTH/2)), uint(r_y-model.BLOCK_WIDTH/2), uint(r_x-int(model.BOCLK_LENTH/2)), uint(r_y-model.BLOCK_WIDTH/2), model.DirLeft)
		// 右垂直边界检测
		flag = flag && FindBlock(uint(r_x+int(model.BOCLK_LENTH/2)), uint(r_y+model.BLOCK_WIDTH/2), uint(r_x+int(model.BOCLK_LENTH/2)), uint(r_y-model.BLOCK_WIDTH/2), model.DirUp)
		// 左垂直边界检测
		flag = flag && FindBlock(uint(r_x-int(model.BOCLK_LENTH/2)), uint(r_y+model.BLOCK_WIDTH/2), uint(r_x-int(model.BOCLK_LENTH/2)), uint(r_y-model.BLOCK_WIDTH/2), model.DirUp)
	} else {
		// 垂直方向检测（旋转 90 度）
		// 左垂直边界检测
		flag = flag && FindBlock(uint(r_x+model.BLOCK_WIDTH/2), uint(r_y+int(model.BOCLK_LENTH/2)), uint(r_x+model.BLOCK_WIDTH/2), uint(r_y-int(model.BOCLK_LENTH/2)), model.DirUp)
		// 右垂直边界检测
		flag = flag && FindBlock(uint(r_x-model.BLOCK_WIDTH/2), uint(r_y+int(model.BOCLK_LENTH/2)), uint(r_x-model.BLOCK_WIDTH/2), uint(r_y-int(model.BOCLK_LENTH/2)), model.DirUp)
		// 上水平边界检测
		flag = flag && FindBlock(uint(r_x+model.BLOCK_WIDTH/2), uint(r_y+int(model.BOCLK_LENTH/2)), uint(r_x-model.BLOCK_WIDTH/2), uint(r_y+int(model.BOCLK_LENTH/2)), model.DirLeft)
		// 下水平边界检测
		flag = flag && FindBlock(uint(r_x+model.BLOCK_WIDTH/2), uint(r_y-int(model.BOCLK_LENTH/2)), uint(r_x-model.BLOCK_WIDTH/2), uint(r_y-int(model.BOCLK_LENTH/2)), model.DirLeft)
	}
	//log.Printf("build wall result: %t", flag)
	return flag
}

func buildwall(r_x, r_y int, dir byte) bool {
	if Trywall(r_x, r_y, dir) {
		if dir == 4 || dir == 6 {
			minY := int(r_y - model.BLOCK_WIDTH/2)
			maxY := int(r_y + model.BLOCK_WIDTH/2)
			minX := int(r_x - int(model.BOCLK_LENTH/2))
			maxX := int(r_x + int(model.BOCLK_LENTH/2))
			for y := minY; y <= maxY; y++ {
				for x := minX; x <= maxX; x++ {
					// 判断是否在边缘
					if isWithinBounds(x, y) {
						model.Map[y][x] = 2
					}
				}
			}
		} else {
			minY := int(r_y - model.BOCLK_LENTH/2)
			maxY := int(r_y + model.BOCLK_LENTH/2)
			minX := int(r_x - int(model.BLOCK_WIDTH/2))
			maxX := int(r_x + int(model.BLOCK_WIDTH/2))
			for y := minY; y <= maxY; y++ {
				for x := minX; x <= maxX; x++ {
					// 判断是否在边缘
					if isWithinBounds(x, y) {
						model.Map[y][x] = 2
					}
				}
			}
		}
		return true
	} else {
		return false
	}
}
func Maprandom() {
	for {
		clearMap()
		model.EdgePoints = make(map[[2]int]byte)
		// 使用泊松盘采样生成随机点
		x0, y0, x1, y1, r := 0.0, 0.0, float64(model.MAP_SIZE_X), float64(model.MAP_SIZE_Y), 200.0
		k := 100

		// 生成点
		points := poissondisc.Sample(x0, y0, x1, y1, r, k, nil)

		// 将点四舍五入到整型并填充到 grid
		for _, p := range points {
			r_x, r_y := int(math.Round(p.X)), int(math.Round(p.Y)) // 关键修改
			dir := rand.Intn(4)*2 + 2
			num := rand.Intn(3) + 2
			dx, dy := getDirectionDelta(byte(dir))
			l := model.BOCLK_LENTH
			w := model.BLOCK_WIDTH
			dir_deg := 0
			if dir == 4 || dir == 6 {
				dx = dx * (model.BOCLK_LENTH + 1)
				dy = dy * (model.BLOCK_WIDTH + 1)
				dir_deg = 8
			} else {
				dx = dx * (model.BLOCK_WIDTH + 1)
				dy = dy * (model.BOCLK_LENTH + 1)
				dir_deg = 4
			}
			block := &model.Block{
				L:   l,
				W:   w,
				Dir: byte(dir_deg),
			}
			log.Printf("[地图生成] 随机生成砖块: (%d,%d),方向：%d %d,step(%d,%d)", r_x, r_y, dir, dir_deg, dx, dy)
			for i := 0; i < num; i++ {
				if buildwall(r_x+dx*i, r_y+dy*i, byte(dir)) {
					block.P = append(block.P, model.MapPoint{X: uint(r_x + dx*i), Y: uint(r_y + dy*i)})
				}
			}
			if len(block.P) != 0 {
				model.Blocks = append(model.Blocks, block)
			}

		}
		if CheckZeroConnectivity() {
			dc := gg.NewContext(int(model.MAP_SIZE_X), int(model.MAP_SIZE_Y))
			dc.SetRGB(1, 1, 1) // 白色背景
			dc.Clear()

			for y := 0; y < int(model.MAP_SIZE_Y); y++ {
				for x := 0; x < int(model.MAP_SIZE_X); x++ {
					switch model.Map[y][x] {
					case 2:
						dc.SetRGB(0, 0, 1) // 蓝色点
						dc.DrawPoint(float64(x), float64(y), 1)
						dc.Fill()
					case 3:
						dc.SetRGB(0, 1, 0) // 绿色点
						dc.DrawPoint(float64(x), float64(y), 1)
						dc.Fill()
					}
				}
			}

			if err := dc.SavePNG("grid_points.png"); err != nil {
				log.Fatal(err)
			}
			break // 满足条件，退出循环
		}
		log.Printf("[地图生成] 生成的地图不满足连通性，重新生成...")
	}
	for _, val := range model.Blocks {
		log.Printf("block: %v", val)
	}
	log.Printf("[地图生成] 地图生成完成，已保存为 grid_points.png")
}
