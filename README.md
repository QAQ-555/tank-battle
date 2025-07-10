# 0 坦克大战后端接口
接口地址```192.168.10.233:8888/ws```
通过websocket，以玩家身份加入游戏，会在游戏中创建一个对应的坦克对象
# 1数据传递方法
```json
{
    "type": 15, //消息类型 byte
    "id": "4e3a8178-4f1b-44ff-959f-197f7ca84979" //消息目标id sting
    "payload": {

    } //传递数据
}
```
当前数据传递均由json进行，约定接收发送数据时使用以上结构的json
tpye进行payload类型的标识
type < 15 服务端发送数据

type == 0 服务端发送提示数据
```
"payload": {
        "notice": "username is empty or already exists"
    }
```
服务端进行相应而发送的文本消息

type == 1 链接建立时发送所需数据，id为目标客户端32字节长度字符串，payload见 [2.1](#21-链接建立)
type == 2 游戏状态广播，id通用字符串如```broadcast message gamer```，payload见[2.2](#22-数据广播)

type >= 15 客户端发送数据
type == 15 坦克操作代码 操作tank时数据，id为发起客户端32字节长度字符串，payload见[3.1](#31-坦克操作指令)

type == 16 注册请求，详见 [2.1](#21-链接建立)

# 2 服务器消息payload
## 2.1 链接建立
通过接口进行websocket链接成功时，客户端会收到
```json
{
    "type": 0,
    "id": "perpartext",
    "payload": {
        "notice": "websocket connect success"
    }
}
```
代表客户端已经成功与服务端建立链接，之后需要在规定时间内发送
```json
{
    "type": 16,
    "id": "",
    "payload": {
        "username":"qaq_555", //string
        "success":true    //发送true以注册
    }
}
```
username为玩家身份的唯一标识

而因为重复等原因注册失败时会有
```json
{
    "type": 0,
    "id": "888",
    "payload": {
        "notice": "username is empty or already exists"
    }
}
```
成功注册时，会收到由客户端发送来的地图与连接相关数据，如下所示
```json
{
    "type": 1,
    "id": "test",
    "payload": {
        "map":"AAAAAA",      //地图字节序列
        "map_size_x": 1542,  //地图尺寸
        "map_size_y": 512,   //
        "tank_coord_x": 1,   //控制坦克坐标
        "tank_coord_y": 1,
        "tank_facing": 2,    //坦克面向
        "tick_interval_ms": 1000, //服务端参数
        "map_render_ms": 1000,
        "username": "test"   //用户名
    }
}
```
客户端需要利用地图数据构建地图用于后续渲染，并保留id用于向服务端发送请求

map为[map_size_y][map_size_x]byte序列化生成，需要利用map_size_y与map_size_x进行反序列化
 (0，0)→(map_size_x,0)
   ↓
 (0,map_size_y)
 地图示意如上

## 2.2 数据广播
成功注册后，每间隔tick_interval_ms后，服务端会广播当前地图上所有元素的状态，客户端利用元素状态与地图构建完整游戏状态
目前发送客户端所存储的完整坦克状态
```json
"payload": {
    "tanks": [
        {
            "x": 1,      //坦克中心x坐标 uint
            "y": 1,      //坦克中心y坐标 uint
            "reload": 0, //坦克开火cd
            "trigger": false, //坦克扳机 bool
            "gunfacing": 2,   //炮管朝向 Dir***
            "status": 1,      //坦克状态 byte 
            "orientation": 5, //移动方向 Dir***
            "id": "ae9f7384-02b5-4407-8bab-4203beffc13a" //链接对应坦克id
        },
        {
            "x": 250,
            "y": 1,
            "reload": 0,
            "trigger": false,
            "gunfacing": 2,
            "status": 1,
            "orientation": 5,
            "id": "1d35960b-0885-45ee-ae70-12cb3c5a75da"
        }
    ]
}
```
### 2.2.1 方向代码
在描述方向时，如orientation gunfacing，会传递如下数据
```go
const (
	DirUp        = 8
	DirUpRight   = 9     //    up
	DirRight     = 6     //    ↑ 
	DirDownRight = 3     //  7 8 9
	DirDown      = 2     //← 4 5 6 → right
	DirDownLeft  = 1     //  1 2 3
	DirLeft      = 4     //    ↓
	DirUpLeft    = 7
	DirNone      = 5
) //8向方位代码
```
约定利用Dir***标识方向，2对应下、8对应上、4对应左、6对应右、5对应静止
（也就是小键盘对应方向）
炮管方向不存在5，设定为最后移动方向
### 2.2.2 坦克状态
```go
const (
	StatusFree  byte = 0
	StatusTaken byte = 1
) //坦克状态
```
# 3 客户端消息

### 3.1 坦克操作指令
客户端需要向所控制坦克发送移动等指令时，需要将payload构建如下
```json
"payload": {
    "up":0, // bool
    "down":1,
    "left":0,
    "right":0,
    "action":"0"//string
}
```
up,down,left,right 代表客户端是否按下对应按键，按下时持续传递1，抬起时持续传递0
action 保留





