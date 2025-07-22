package model

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var FlagChan = make(chan bool)

const (
	MAP_SIZE_X          uint = 1542
	MAP_SIZE_Y          uint = 512
	TICK_INTERVAL_MS         = 5000
	MAP_RENDER_MS            = 5000
	WAIT_REPLY_TIME          = 60
	TANK_RELOAD_SECONDS      = 3
	BOCLK_LENTH              = 20 //must even
	BLOCK_WIDTH              = 10
) //建立链接发送数据

var TANK_RELOAD_VALUE = TANK_RELOAD_SECONDS * 1000 / MAP_RENDER_MS * 5

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

// 发送消息类型常量定义，值小于 0x0f
const (
	TypeMessage         byte = 0    // type=0 提示消息
	TypeInitData        byte = 1    // type=1 连接建立后初始化数据
	TypeGameStatus      byte = 0x02 // type=2 游戏状态广播
	TypeShotEvent       byte = 3    // type=3 射击事件广播
	TypeErrorNotice     byte = 4    // type=4 错误提示
	TypeTankChangeEvent byte = 5    // type=5 坦克变化事件广播
	TypeHitEvent        byte = 7    // type=7 命中事件广播
	TypeEmojiC          byte = 8    // type=8 客户端自定义表情
)

// 接收消息类型常量定义，值大于等于 0x0f
const (
	TypeTankOperation   byte = 0x0f // type=15 坦克操作指令
	TypeRegisterRequest byte = 0x10 // type=16 注册请求
	TypeHitNotice       byte = 0x11 // type=17 命中通知
	TypeRespawnRequest  byte = 0x12 // type=18 复活请求
	TypeEmojiS          byte = 0x13 // type=19 服务端转发自定义表情
)

var UP = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
} //websocket设置

var (
	Clients       = make(map[string]*Client)
	ClientsMu     sync.Mutex
	SpawnTanks    []*Tank
	SpawnTanksMu  sync.Mutex
	ShotEvents    []*ShotEvent
	ShotEventsMu  sync.Mutex
	Shotedtanks   []*Tank
	ShotedtanksMu sync.Mutex
	Usernames     []string
	UsernameMu    sync.Mutex
	EdgePoints    = make(map[[2]int]byte)
	Blocks        []*Block
	BlocksMu      sync.Mutex
)

type MapPoint struct {
	X uint `json:"x"`
	Y uint `json:"y"`
}

type Block struct {
	P   []MapPoint `json:"blockpoints"`
	L   int        `json:"length"`
	W   int        `json:"width"`
	Dir byte       `json:"dir"`
}

// 通信壳
type WebMessage struct {
	Type byte `json:"type"`
	//TimeStamp int64       `json:"time"`
	ID      string      `json:"id"`
	Payload interface{} `json:"payload"`
}

// 发送地图信息
type MapConfig struct {
	Map          []*Block `json:"map"`
	MapSizeX     uint     `json:"map_size_x"`
	MapSizeY     uint     `json:"map_size_y"`
	TankCoordX   uint     `json:"tank_coord_x"`
	TankCoordY   uint     `json:"tank_coord_y"`
	Tankfacing   byte     `json:"tank_facing"`
	TickInterval int      `json:"tick_interval_ms"`
	MapRenderMS  int      `json:"map_render_ms"`
	ServerID     string   `json:"username"`
	Tanks        []*Tank  `json:"tanks"`
	Blocks       []*Block `json:"-"`
}

// 坦克状态
type Tank struct {
	LocalX      uint   `json:"x"`
	LocalY      uint   `json:"y"`
	Reload      uint   `json:"reload"`
	Trigger     bool   `json:"trigger"`
	GunFacing   byte   `json:"gunfacing"`
	Status      byte   `json:"status"`
	Orientation byte   `json:"orientation"`
	ID          string `json:"username"`
	Point       int    `json:"point"`
}

// 游戏状态
type GameState struct {
	Tanks      []*Tank      `json:"tanks"`
	ShotEvents []*ShotEvent `json:"ShotEvents,omitempty"`
	Map        []byte       `json:"map,omitempty"`
	//Items   []*Item   `json:"items,omitempty"`

}

// 发射活动
type ShotEvent struct {
	Tank   string `json:"username"`
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
	WriteMutex sync.Mutex // 添加写互斥锁
}

// 客户端请求
type OperatePayload struct {
	Up     bool
	Down   bool
	Left   bool
	Right  bool
	Action string
}

type HitPayload struct {
	Username string `json:"username"`
	Victim   string `json:"victim"`
}

type RequestPayload struct {
	Username string `json:"username"`
	Success  bool   `json:"success"`
}

type NoticePayload struct {
	Notice string `json:"notice"`
}

type RespawnPayload struct {
	Username string `json:"username"`
	Success  bool   `json:"success"`
}

type TankChangePayload struct {
	Username string `json:"username"`
	TurnTo   bool   `json:"turnto"`
	X        uint   `json:"x"`
	Y        uint   `json:"y"`
}

// 地图数据
var Map [MAP_SIZE_Y][MAP_SIZE_X]byte
