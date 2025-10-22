# WASM Memory Scanner

类似 Cheat Engine 的 WASM 内存扫描工具，可以在浏览器中搜索和修改 WebAssembly 程序的内存。

## 🎯 功能特性

- ✅ 首次扫描：在整个 WASM 内存中搜索指定值
- ✅ 继续扫描：在上次结果中过滤，快速定位目标地址
- ✅ 多种数据类型：支持 int8/16/32, uint8/16/32, float32/64
- ✅ 灵活的比较模式：精确值、已改变、未改变、增加、减少
- ✅ 内存修改：直接修改找到的内存地址
- ✅ 实时监控：监控多个地址的实时变化
- ✅ 十六进制转储：查看原始内存数据
- ✅ 图形化界面：提供友好的浏览器界面

## 📦 文件说明

```
wasm/
├── memory-scanner.js      # 核心扫描库
├── usage-example.js       # 使用示例代码
├── browser-scanner.html   # 浏览器图形界面
└── README.md             # 本文档
```

## 🚀 快速开始

### 方法 1: 使用图形界面（推荐）

1. 在浏览器中打开 `browser-scanner.html`
2. 打开浏览器控制台（F12）
3. 找到你的 WASM 实例的 memory 对象
4. 运行：
   ```javascript
   attachScanner(wasmInstance.exports.memory);
   ```
5. 开始使用图形界面进行扫描！

### 方法 2: 在控制台中使用

```javascript
// 1. 加载扫描器
const scanner = new WasmMemoryScanner(wasmInstance.exports.memory);

// 2. 首次扫描（假设当前 HP 是 1000）
scanner.firstScan(1000, 'int32');
// 输出: Found 15234 results

// 3. 继续扫描（HP 变成 950）
scanner.nextScan(950, 'exact');
// 输出: Filtered to 234 results

// 4. 再次扫描（HP 变成 1000）
scanner.nextScan(1000, 'exact');
// 输出: Filtered to 2 results

// 5. 查看结果
const results = scanner.getResults();
console.log(results);
// [
//   { address: 0x12a4c0, value: 1000, type: 'int32' },
//   { address: 0x45f830, value: 1000, type: 'int32' }
// ]

// 6. 修改内存
scanner.writeValue(results[0].address, 9999, 'int32');
```

## 🎮 实战案例

### 案例 1: 查找角色血量 HP

```javascript
// 当前 HP: 1000
scanner.firstScan(1000, 'int32');

// 受到伤害，HP: 950
scanner.nextScan(950, 'exact');

// 使用药水，HP: 1000
scanner.nextScan(1000, 'exact');

// 再次受伤，HP: 800
scanner.nextScan(800, 'exact');

// 找到唯一地址
const hpAddress = scanner.getResults()[0];
console.log('HP 地址:', hpAddress);

// 修改为无敌血量
scanner.writeValue(hpAddress.address, 999999, 'int32');
```

### 案例 2: 查找金币数量

```javascript
scanner.reset(); // 重置搜索

// 当前金币: 5000
scanner.firstScan(5000, 'uint32');

// 买东西后: 4800
scanner.nextScan(4800, 'exact');

// 卖东西后: 5300
scanner.nextScan(5300, 'exact');

// 找到金币地址
const goldAddress = scanner.getResults()[0];

// 修改金币
scanner.writeValue(goldAddress.address, 999999, 'uint32');
```

### 案例 3: 查找浮点数（经验值百分比）

```javascript
scanner.reset();

// 当前经验: 75.5%
scanner.firstScan(75.5, 'float32');

// 杀怪后: 76.2%
scanner.nextScan(76.2, 'exact');

// 再杀怪: 78.9%
scanner.nextScan(78.9, 'exact');

// 找到经验地址
const expAddress = scanner.getResults()[0];
```

### 案例 4: 不知道具体数值的情况

```javascript
scanner.reset();

// 假设你不知道当前生命值是多少
// 先扫描一次所有值（这会很慢，但建立基准）
scanner.firstScan(0, 'int32');

// 受到伤害后，搜索"减少"的值
scanner.scanDecreased();

// 再次受伤，继续搜索"减少"的值
scanner.scanDecreased();

// 使用药水，搜索"增加"的值
scanner.scanIncreased();

// 不动，搜索"未变化"的值
scanner.scanUnchanged();
```

### 案例 5: 实时监控多个地址

```javascript
const addresses = [
  { address: 0x12a4c0, type: 'int32' },  // HP
  { address: 0x12a4c4, type: 'int32' },  // MP
  { address: 0x45f830, type: 'uint32' }, // Gold
];

// 每秒监控一次
const watchId = scanner.watchAddresses(addresses, 1000);

// 停止监控
scanner.stopWatch(watchId);
```

### 案例 6: 内存十六进制转储

```javascript
// 查看某个地址周围的内存
scanner.hexDump(0x12a4c0, 256);

// 输出示例:
// 0x0012a4c0  e8 03 00 00 dc 05 00 00 10 27 00 00 00 00 00 00  .........'......
// 0x0012a4d0  01 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00  ................
```

## 🔧 API 参考

### WasmMemoryScanner 类

#### 构造函数
```javascript
const scanner = new WasmMemoryScanner(wasmMemory);
```

#### 方法

**firstScan(value, type, littleEndian)**
- 首次扫描，搜索整个内存
- `value`: 要搜索的值
- `type`: 数据类型（'int8', 'uint8', 'int16', 'uint16', 'int32', 'uint32', 'float32', 'float64'）
- `littleEndian`: 字节序，默认 true（小端）
- 返回: 找到的地址列表

**nextScan(value, compareType)**
- 继续扫描，过滤上次的结果
- `value`: 新的值
- `compareType`: 比较类型（'exact', 'changed', 'unchanged', 'increased', 'decreased'）
- 返回: 过滤后的地址列表

**scanChanged() / scanUnchanged() / scanIncreased() / scanDecreased()**
- 快捷方法，搜索特定变化模式的值

**readValue(address, type, littleEndian)**
- 读取指定地址的值
- 返回: 读取到的值

**writeValue(address, value, type, littleEndian)**
- 写入指定地址的值

**watchAddresses(addresses, interval)**
- 监控多个地址的实时变化
- `addresses`: 地址数组 `[{address, type}, ...]`
- `interval`: 更新间隔（毫秒）
- 返回: 定时器 ID

**stopWatch(intervalId)**
- 停止监控

**hexDump(address, length)**
- 显示内存的十六进制转储
- `address`: 起始地址
- `length`: 转储长度

**reset()**
- 重置搜索结果

**getResults(limit)**
- 获取当前搜索结果
- `limit`: 返回的最大数量，默认 100

**exportResults(filename)**
- 导出搜索结果到 JSON

## 💡 高级技巧

### 1. 查找指针链

```javascript
// 找到基础地址
scanner.firstScan(someValue, 'int32');
// ... 多次扫描

// 读取该地址作为指针
const baseAddress = scanner.getResults()[0].address;
const pointerValue = scanner.readValue(baseAddress, 'uint32');

// 跟随指针，读取偏移位置的值
const actualValue = scanner.readValue(pointerValue + 0x10, 'int32');
```

### 2. 搜索字节序列（特征码）

```javascript
function searchSignature(scanner, pattern) {
  scanner.updateMemoryView();
  const buffer = scanner.buffer;
  const results = [];

  for (let i = 0; i <= buffer.length - pattern.length; i++) {
    let match = true;
    for (let j = 0; j < pattern.length; j++) {
      // -1 表示通配符
      if (pattern[j] !== -1 && buffer[i + j] !== pattern[j]) {
        match = false;
        break;
      }
    }
    if (match) results.push(i);
  }

  return results;
}

// 使用通配符搜索
const signature = [0x48, 0x89, 0x5C, 0x24, -1, 0x48, 0x89, 0x74];
const addresses = searchSignature(scanner, signature);
```

### 3. 批量修改

```javascript
function modifyAllResults(scanner, newValue) {
  const results = scanner.getResults();
  for (const result of results) {
    scanner.writeValue(result.address, newValue, result.type);
  }
  console.log(`Modified ${results.length} addresses`);
}

modifyAllResults(scanner, 9999);
```

### 4. 自动化脚本

```javascript
// 自动保持血量满值
setInterval(() => {
  const currentHP = scanner.readValue(hpAddress, 'int32');
  if (currentHP < 1000) {
    scanner.writeValue(hpAddress, 1000, 'int32');
    console.log('HP restored to 1000');
  }
}, 1000);
```

## 🔍 如何找到 WASM Memory

### 方法 1: 从全局变量查找
```javascript
// 查找所有 WebAssembly.Memory 实例
for (let key in window) {
  try {
    if (window[key] instanceof WebAssembly.Memory) {
      console.log('Found memory at:', key);
      attachScanner(window[key]);
    }
  } catch (e) {}
}
```

### 方法 2: 拦截 WebAssembly.instantiate
```javascript
const originalInstantiate = WebAssembly.instantiate;
WebAssembly.instantiate = async function(...args) {
  const result = await originalInstantiate.apply(this, args);
  if (result.instance && result.instance.exports.memory) {
    console.log('Intercepted WASM memory:', result.instance.exports.memory);
    window.wasmMemory = result.instance.exports.memory;
    attachScanner(window.wasmMemory);
  }
  return result;
};
```

### 方法 3: 从 Emscripten 模块
```javascript
// 如果使用 Emscripten 编译
if (typeof Module !== 'undefined' && Module.HEAPU8) {
  const memory = Module.HEAPU8.buffer;
  attachScanner(new WebAssembly.Memory({
    initial: memory.byteLength / 65536
  }));
}
```

## ⚠️ 注意事项

1. **性能**: 首次扫描整个内存可能需要几秒钟，特别是在大型程序中
2. **内存增长**: WASM 内存可能会动态增长，扫描器会自动更新视图
3. **字节序**: 大多数系统使用小端字节序，但某些情况可能需要大端
4. **类型选择**: 选择正确的数据类型很重要，错误的类型可能找不到值
5. **合法性**: 仅用于学习和单机游戏，不要用于破坏在线游戏平衡

## 🎓 学习资源

- [WebAssembly Memory](https://developer.mozilla.org/en-US/docs/WebAssembly/JavaScript_interface/Memory)
- [DataView API](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/DataView)
- [Cheat Engine Tutorial](https://www.cheatengine.org/tutorial.php)

## 📝 许可证

MIT License - 仅供学习和研究使用

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

---

**祝你游戏愉快！记得负责任地使用这个工具。** 🎮
