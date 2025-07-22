import threading
import time
import websocket
import json
import random
import string

# 服务器WebSocket地址（根据实际情况修改）
WS_URL = "ws://localhost:8888/ws"
# 并发连接数
CONCURRENT_CONNECTIONS = 500
# 每个连接的生命周期（秒）
CONNECTION_LIFETIME = 0.5

def random_username(length=8):
    """生成随机用户名"""
    letters = string.ascii_letters + string.digits
    return ''.join(random.choice(letters) for _ in range(length))

def websocket_client():
    """WebSocket客户端函数"""
    username = random_username()
    ws = None
    try:
        # 建立WebSocket连接
        ws = websocket.WebSocket()
        ws.connect(WS_URL)
        print(f"已建立连接: {username}")

        # 随机选择断开阶段
        stage = random.choice([1, 2, 3])

        if stage == 1:
            # 阶段1：刚连接就断开
            print(f"{username} 阶段1断开")
            return

        # 阶段2/3：发送用户名验证消息
        auth_msg = {
            "Type": 16,
            "ID": username,
            "Payload": {
                "Username": username,
                "Success": True
            }
        }
        ws.send(json.dumps(auth_msg))

        if stage == 2:
            # 阶段2：发送完用户名就断开
            print(f"{username} 阶段2断开")
            return

        # 阶段3：正常等待后断开
        time.sleep(CONNECTION_LIFETIME)

    except Exception as e:
        print(f"连接 {username} 发生错误: {str(e)}")
    finally:
        if ws:
            ws.close()
            print(f"已关闭连接: {username}")

def main():
    """主函数：创建并发连接"""
    threads = []

    # 创建并发连接
    for i in range(CONCURRENT_CONNECTIONS):
        thread = threading.Thread(target=websocket_client)
        threads.append(thread)
        thread.start()
        # 稍微错开连接时间，避免瞬间峰值
        time.sleep(0.01)

    # 等待所有线程完成
    for thread in threads:
        thread.join()

    print("压力测试完成")

if __name__ == "__main__":
    main()