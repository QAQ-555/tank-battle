package model

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var FlagChan = make(chan bool)

const (
	MAP_SIZE_X       uint = 1542
	MAP_SIZE_Y       uint = 512
	TICK_INTERVAL_MS      = 50
	MAP_RENDER_MS         = 50
	WAIT_REPLY_TIME       = 60
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
	Clients      = make(map[string]*Client)
	ClientsMu    sync.Mutex
	SpawnTanks   []*Tank
	SpawnTanksMu sync.Mutex
	ShotEvents   []*ShotEvent
	ShotEventsMu sync.Mutex
	Usernames    []string
	UsernameMu   sync.Mutex
)

// 通信壳
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
	TankCoordX   uint   `json:"tank_coord_x"`
	TankCoordY   uint   `json:"tank_coord_y"`
	Tankfacing   byte   `json:"tank_facing"`
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
	Tanks      []*Tank      `json:"tanks"`
	ShotEvents []*ShotEvent `json:"ShotEvents,omitempty"`
	Map        []byte       `json:"map,omitempty"`
	//Items   []*Item   `json:"items,omitempty"`

}

// 子弹状态
type ShotEvent struct {
	Tank   string `json:"shoter"`
	LocalX uint   `json:"x"`
	LocalY uint   `json:"y"`
	Facing byte   `json:"orientation"`
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
