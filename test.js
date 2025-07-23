let root;

// 加载 proto 文件
async function loadProto() {
    try {
        root = await protobuf.load([
            "./pb/pack.proto",
            "./pb/payload/payload.proto"
        ]);
        const msgPackType = root.lookupType("msgpack.MsgPack");
        const payloadOneof = msgPackType.oneofsArray.find(oneof => oneof.name === "payload");
        const messageTypeSelect = document.getElementById("messageType");

        payloadOneof.fieldsArray.forEach(field => {
            const option = document.createElement("option");
            option.value = field.type;
            option.textContent = field.type;
            messageTypeSelect.appendChild(option);
        });
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
    const headers = ["字段名", "字段类型", "字段编号"];

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

        fieldNameCell.textContent = field.name;
        fieldTypeCell.textContent = field.type;
        fieldNumberCell.textContent = field.id;

        row.appendChild(fieldNameCell);
        row.appendChild(fieldTypeCell);
        row.appendChild(fieldNumberCell);
        tbody.appendChild(row);
    });

    table.appendChild(tbody);
    tableContainer.innerHTML = "";
    tableContainer.appendChild(table);
}

// 页面加载完成后加载 proto 文件
window.onload = function () {
    loadProto();
};