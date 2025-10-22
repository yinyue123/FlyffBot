/**
 * WASM Memory Scanner 使用示例
 * 演示如何使用类似 Cheat Engine 的方式查找内存地址
 */

// 假设你已经加载了 WASM 模块
// const wasmInstance = await WebAssembly.instantiate(...);
// const scanner = new WasmMemoryScanner(wasmInstance.exports.memory);

// ============================================
// 场景 1: 查找角色的血量 HP
// ============================================

// 步骤 1: 假设当前 HP 是 1000，首次扫描
scanner.firstScan(1000, 'int32');
// 输出: Found 15234 results (可能有很多匹配的地址)

// 步骤 2: 受到伤害，HP 变成 950，继续搜索
scanner.nextScan(950, 'exact');
// 输出: Filtered to 234 results

// 步骤 3: 使用药水，HP 变成 1000，再次搜索
scanner.nextScan(1000, 'exact');
// 输出: Filtered to 12 results

// 步骤 4: 再次受伤，HP 变成 800
scanner.nextScan(800, 'exact');
// 输出: Filtered to 2 results

// 查看最终结果
const results = scanner.getResults();
console.log('Found HP addresses:', results);
// 输出:
// [
//   { address: 0x12a4c0, value: 800, type: 'int32' },
//   { address: 0x45f830, value: 800, type: 'int32' }
// ]

// 尝试修改第一个地址看看效果
scanner.writeValue(results[0].address, 9999, 'int32');

// ============================================
// 场景 2: 查找金币数量
// ============================================

// 重置搜索
scanner.reset();

// 当前金币: 5000
scanner.firstScan(5000, 'uint32');

// 买东西花了 200，现在 4800
scanner.nextScan(4800, 'exact');

// 卖东西赚了 500，现在 5300
scanner.nextScan(5300, 'exact');

// 找到金币地址
const goldResults = scanner.getResults();
console.log('Gold address:', goldResults);

// ============================================
// 场景 3: 查找浮点数（比如经验值百分比）
// ============================================

scanner.reset();

// 当前经验 75.5%
scanner.firstScan(75.5, 'float32');

// 杀怪后 76.2%
scanner.nextScan(76.2, 'exact');

// 再杀怪 78.9%
scanner.nextScan(78.9, 'exact');

// ============================================
// 场景 4: 不知道具体数值，但知道变化规律
// ============================================

scanner.reset();

// 首次扫描任意值（比如搜索所有 uint32）
scanner.firstScan(0, 'uint32'); // 这会扫描所有内存

// 做一些操作，然后搜索"增加"的值
scanner.scanIncreased();

// 再做操作，搜索"继续增加"的值
scanner.scanIncreased();

// 或者搜索"减少"的值
scanner.scanDecreased();

// 或者搜索"未变化"的值
scanner.scanUnchanged();

// ============================================
// 场景 5: 监控多个地址的实时变化
// ============================================

const addressesToWatch = [
  { address: 0x12a4c0, type: 'int32' },  // HP
  { address: 0x12a4c4, type: 'int32' },  // MP
  { address: 0x45f830, type: 'uint32' }, // Gold
];

// 每秒监控一次
const watchId = scanner.watchAddresses(addressesToWatch, 1000);

// 停止监控
// scanner.stopWatch(watchId);

// ============================================
// 场景 6: 内存十六进制转储（查看原始内存）
// ============================================

// 查看某个地址周围的内存
scanner.hexDump(0x12a4c0, 256);
// 输出类似:
// 0x0012a4c0  e8 03 00 00 dc 05 00 00 10 27 00 00 00 00 00 00  .........'......
// 0x0012a4d0  01 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00  ................

// ============================================
// 高级技巧: 查找指针链
// ============================================

// 1. 找到基础地址（比如玩家对象）
scanner.firstScan(playerValue, 'int32');
// ... 多次扫描缩小范围

// 2. 读取该地址作为指针
const baseAddress = scanner.getResults()[0].address;
const pointerValue = scanner.readValue(baseAddress, 'uint32');
console.log(`Pointer value: 0x${pointerValue.toString(16)}`);

// 3. 跟随指针，读取偏移位置的值
const actualValue = scanner.readValue(pointerValue + 0x10, 'int32');
console.log(`Value at pointer + 0x10: ${actualValue}`);

// ============================================
// 实用函数: 批量修改
// ============================================

function modifyAllResults(scanner, newValue) {
  const results = scanner.getResults();
  for (const result of results) {
    scanner.writeValue(result.address, newValue, result.type);
  }
  console.log(`Modified ${results.length} addresses to ${newValue}`);
}

// ============================================
// 实用函数: 搜索字节序列（比如特征码）
// ============================================

function searchBytes(scanner, bytePattern) {
  scanner.updateMemoryView();
  const buffer = scanner.buffer;
  const results = [];

  for (let i = 0; i <= buffer.length - bytePattern.length; i++) {
    let match = true;
    for (let j = 0; j < bytePattern.length; j++) {
      if (bytePattern[j] !== -1 && buffer[i + j] !== bytePattern[j]) {
        match = false;
        break;
      }
    }
    if (match) {
      results.push(i);
    }
  }

  return results;
}

// 使用通配符搜索特征码（-1 表示任意字节）
const signature = [0x48, 0x89, 0x5C, 0x24, -1, 0x48, 0x89, 0x74];
const sigResults = searchBytes(scanner, signature);
console.log('Found signature at:', sigResults);
