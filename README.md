# 0 数据接收
## 移动，行动
```json
{
    "up":0,
    "down":0,
    "left":0,
    "right":1,
    "action":"0"
```
# 1 坦克大战接口
```json
{
    "type": 3, //消息类型 byte
    "id"
    "payload": {

    } //传递数据
}
```
当前数据传递均由json进行，服务端发送数据时会发送以上结构的数据
type == 1 链接建立时发送所需数据，见 [1.1](#11-链接建立)
type == 2 游戏状态广播，见[1.2](#12-数据广播)

## 1.1 链接建立
通过ws://{ip}:8888/ws建立websocket链接
链接成功payload会有
```json
{
    "map": "AQEBAAAAAAAAAAAAAAAAAAAAAAAAAAAA", //地图数据字节序列 [][]byte
    "map_size_x": 30,       //地图x轴长度   uint
    "map_size_y": 30,       //地图y轴长度   uint
    "tick_interval_ms": 50, //服务端广播数据频率 单位ms
    "map_render_ms": 500,   //服务端数据刷新频率 单位ms
    "ws_server_id": "966910f3-7bfc-4c3c-8ae9-7493f0bb2e69" //链接对应id string
}
```
map为[map_size_y][map_size_x]byte序列化生成，需要利用map_size_y与map_size_x进行反序列化
 (0，0)→(map_size_x,0)
   ↓
 (0,map_size_y)
 地图示意如上

当ws_server_id为viewer
## 1.2 数据广播

```json
{
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
    ],
    "bullet":null,  //子弹
    "item":null     //物品
}
```
返回如上的json组
### 1.2.1 方向代码
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
设定如上，利用Dir***标识方向，2对应下、8对应上、4对应左、6对应右、5对应禁止
（也就是小键盘对应方向）
炮管方向不存在5，设定为最后移动方向
### 1.2.2 坦克状态
```go
const (
	StatusFree  byte = 0
	StatusTaken byte = 1
) //坦克状态
```
#### bullet
保留
#### item 
保留






