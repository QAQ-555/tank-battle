// ... 已有代码 ...
let socket;
let root;
let MsgTypes = {};

// 加载 proto 文件
async function loadProto() {
    try {
        root = await protobuf.load([
            "./pb/pack.proto",
            "./pb/payload/payload.proto"
        ]);
        const msgPackType = root.lookupType("msgpack.MsgPack");
        // 将 msgpack.MsgPack 类型赋值给 MsgTypes.MsgPack
        MsgTypes.MsgPack = msgPackType; 
        const payloadOneof = msgPackType.oneofsArray.find(oneof => oneof.name === "payload");
        const messageTypeSelect = document.getElementById("messageType");

        payloadOneof.fieldsArray.forEach(field => {
            const option = document.createElement("option");
            option.value = field.type;
            option.textContent = field.type;
            messageTypeSelect.appendChild(option);
        });
        console.log("Proto 文件加载成功");
    } catch (error) {
        console.error("Proto 文件加载失败:", error);
    }
}

// 生成表格
function generateTable() {
    const messageTypeSelect = document.getElementById("messageType");
    const selectedType = messageTypeSelect.value;
    const tableContainer = document.getElementById("tableContainer");

    if (!selectedType) {
        tableContainer.innerHTML = "";
        return;
    }

    const type = root.lookupType(selectedType);
    const table = document.createElement("table");
    const thead = document.createElement("thead");
    const headerRow = document.createElement("tr");
    const headers = ["字段名", "字段类型", "字段编号", "数值"];

    headers.forEach(headerText => {
        const th = document.createElement("th");
        th.textContent = headerText;
        headerRow.appendChild(th);
    });

    thead.appendChild(headerRow);
    table.appendChild(thead);

    const tbody = document.createElement("tbody");
    type.fieldsArray.forEach(field => {
        const row = document.createElement("tr");
        const fieldNameCell = document.createElement("td");
        const fieldTypeCell = document.createElement("td");
        const fieldNumberCell = document.createElement("td");
        const fieldValueCell = document.createElement("td");
        const input = document.createElement("input");
        input.type = "text";
        input.id = `field-${field.name}`;

        fieldNameCell.textContent = field.name;
        fieldTypeCell.textContent = field.type;
        fieldNumberCell.textContent = field.id;
        fieldValueCell.appendChild(input);

        row.appendChild(fieldNameCell);
        row.appendChild(fieldTypeCell);
        row.appendChild(fieldNumberCell);
        row.appendChild(fieldValueCell);
        tbody.appendChild(row);
    });

    table.appendChild(tbody);
    tableContainer.innerHTML = "";
    tableContainer.appendChild(table);
}

// 发送消息
async function sendMessage() {
    const messageTypeSelect = document.getElementById("messageType");
    const typeInput = document.getElementById("typeInput");
    
    const selectedType = messageTypeSelect.value;
    if (!selectedType) {
        console.error("请选择消息类型");
        return;
    }

    const payloadType = root.lookupType(selectedType);
    if (!payloadType) {
        console.error("未找到对应的消息类型");
        return;
    }

    const msgPackType = root.lookupType("msgpack.MsgPack");
    if (!msgPackType) {
        console.error("未找到 MsgPack 类型");
        return;
    }

    const payloadFieldName = getPayloadFieldName(payloadType, msgPackType);
    if (!payloadFieldName) {
        console.error("未找到匹配的 oneof 字段名");
        return;
    }

    const typeValue = parseInt(typeInput.value, 10);
    if (isNaN(typeValue) || typeValue < 0 || typeValue > 0xff) {
        console.error("Type 字段值无效，请输入 0 - 0xff 之间的整数");
        return;
    }
    const typeBytes = new Uint8Array([typeValue]);

    const msg1 = msgPackType.create({ 
        type: typeBytes,
        target: "test",
        [payloadFieldName] :{}
    });

    const payloadData = payloadType.create({});
    payloadType.fieldsArray.forEach(field => {
        const input = document.getElementById(`field-${field.name}`);
        if (input) {
            const value = input.value;
            switch (field.type) {
                case "bytes":
                    const byteArray = [];
                    value.split(',').forEach(byteStr => {
                        const byte = parseInt(byteStr.trim(), 10);
                        if (!isNaN(byte) && byte >= 0 && byte <= 255) {
                            byteArray.push(byte);
                        }
                    });
                    payloadData[field.name] = new Uint8Array(byteArray);
                    break;
                case "bool":
                    // 处理 bool 类型，将 "true" 转换为 true，"false" 转换为 false
                    payloadData[field.name] = value.toLowerCase() === 'true';
                    break;
                default:
                    payloadData[field.name] = value;
                    break;
            }
        }
    });
    msg1[payloadFieldName] = payloadData;
    console.log("发送消息",msg1);
    const encodedMessage = MsgTypes.MsgPack.encode(msg1).finish();

    socket.send(encodedMessage);
    console.log("消息已发送");
}

// 连接到服务器
function connectToServer() {
    const serverUrl = document.getElementById("serverUrl").value;
    socket = new WebSocket(serverUrl);

    socket.onopen = function () {
        console.log("已连接到服务器");
    };

    socket.onmessage = function (event) {
        if (event.data instanceof Blob) {
            const reader = new FileReader();
            reader.onload = function () {
                const arrayBuffer = reader.result;
                const data = new Uint8Array(arrayBuffer);
                try {
                    // 尝试使用 MsgPack 类型解析数据
                    const message = MsgTypes.MsgPack.decode(data);
                    const messageContent = document.getElementById("messageContent");
                    // 将解析后的消息转换为 JSON 格式
                    const jsonMessage = MsgTypes.MsgPack.toObject(message, {
                        longs: String,
                        enums: String,
                        bytes: String,
                    });
                    messageContent.textContent = JSON.stringify(jsonMessage, null, 2);
                    console.log("解析后的消息:", jsonMessage);
                } catch (error) {
                    console.error("消息解析失败:", error);
                    const messageContent = document.getElementById("messageContent");
                    messageContent.textContent = `消息解析失败: ${error.message}`;
                }
            };
            reader.readAsArrayBuffer(event.data);
        } else {
            console.error("Unexpected data type:", typeof event.data);
        }
    };

    socket.onclose = function () {
        console.log("连接已关闭");
    };

    socket.onerror = function (error) {
        console.error("连接出错:", error);
    };
}

/**
 * 根据 payloadType 和 msgPackType 自动找到对应的 oneof 字段名
 * （通过比较 fields[field].type 和 payloadType.name 或 fullName）
 * 
 * @param {protobuf.Type} payloadType - 用户选择的 payload 类型
 * @param {protobuf.Type} msgPackType - MsgPack 类型
 * @param {string} oneofName - MsgPack 里 oneof 的名字（默认为 "payload"）
 * @returns {string|null} - 对应的字段名（例如 "emoji"），找不到返回 null
 */
function getPayloadFieldName(payloadType, msgPackType, oneofName = "payload") {
    const oneof = msgPackType.oneofsArray.find(o => o.name === oneofName);
    if (!oneof) {
        console.error(`MsgPack 中未找到 oneof: ${oneofName}`);
        return null;
    }

    for (const fieldName of oneof.oneof) {
        const field = msgPackType.fields[fieldName];
        if (!field) continue;

        // 注意这里 field.type 是类型的名字（可能不含包名）
        if (field.type === payloadType.name || field.type === payloadType.fullName || field.type.endsWith(payloadType.name)) {
            console.log(`找到匹配字段: ${fieldName}`);
            return fieldName;
        }
    }

    console.error(`未在 MsgPack.oneof(${oneofName}) 中找到匹配的字段`);
    return null;
}


// 页面加载完成后加载 proto 文件
window.onload = function () {
    loadProto();
};