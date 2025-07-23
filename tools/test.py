import asyncio
import websockets

from pack_pb2 import MsgPack
from payload import payload_pb2

async def send_message(ws):
    """
    从终端读取用户输入，构造 protobuf 并发送
    """
    while True:
        text = await asyncio.get_event_loop().run_in_executor(None, input, "> 输入 target: ")

        msg = MsgPack()
        msg.type = b'\x01'
        msg.target = text

        # 填充一个 emoji payload
        msg.emoji.CopyFrom(payload_pb2.emojipayload(
            emoji_code="😊"
        ))

        data = msg.SerializeToString()
        await ws.send(data)
        print("已发送消息")

async def recv_message(ws):
    """
    接收消息，反序列化并打印
    """
    while True:
        data = await ws.recv()
        if isinstance(data, str):
            print("收到文本消息:", data)
        else:
            msg = MsgPack()
            msg.ParseFromString(data)
            print("收到二进制消息:")
            print(msg)

async def main():
    uri = "ws://localhost:8080/ws"

    async with websockets.connect(uri) as ws:
        print(f"已连接到 {uri}")
        # 并发收发
        await asyncio.gather(
            send_message(ws),
            recv_message(ws),
        )

if __name__ == "__main__":
    asyncio.run(main())
