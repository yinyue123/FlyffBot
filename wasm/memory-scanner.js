/**
 * WASM Memory Scanner - 类似 Cheat Engine 的内存搜索工具
 * 用于在 WASM 内存中搜索特定值并追踪内存地址
 */

class WasmMemoryScanner {
  constructor(wasmMemory) {
    this.memory = wasmMemory;
    this.buffer = null;
    this.dataView = null;
    this.searchResults = [];
    this.updateMemoryView();
  }

  /**
   * 更新内存视图（WASM 内存可能会增长）
   */
  updateMemoryView() {
    this.buffer = new Uint8Array(this.memory.buffer);
    this.dataView = new DataView(this.memory.buffer);
  }

  /**
   * 首次搜索 - 在整个内存中搜索指定值
   * @param {number} value - 要搜索的值
   * @param {string} type - 数据类型: 'int8', 'uint8', 'int16', 'uint16', 'int32', 'uint32', 'float32', 'float64'
   * @param {boolean} littleEndian - 字节序（默认小端）
   * @returns {Array} 找到的地址列表
   */
  firstScan(value, type = 'int32', littleEndian = true) {
    console.time('First Scan');
    this.updateMemoryView();
    this.searchResults = [];

    const memorySize = this.buffer.length;
    const typeInfo = this._getTypeInfo(type);

    // 遍历内存，按类型大小步进
    for (let address = 0; address <= memorySize - typeInfo.size; address++) {
      try {
        const memValue = this._readValue(address, type, littleEndian);

        // 比较值（对于浮点数使用容差）
        if (this._compareValues(memValue, value, type)) {
          this.searchResults.push({
            address: address,
            value: memValue,
            type: type
          });
        }
      } catch (e) {
        // 忽略越界错误
      }
    }

    console.timeEnd('First Scan');
    console.log(`Found ${this.searchResults.length} results`);
    return this.searchResults;
  }

  /**
   * 后续搜索 - 在上次结果中继续搜索
   * @param {number} value - 新的值
   * @param {string} compareType - 比较类型: 'exact', 'changed', 'unchanged', 'increased', 'decreased'
   * @returns {Array} 过滤后的地址列表
   */
  nextScan(value, compareType = 'exact') {
    console.time('Next Scan');
    this.updateMemoryView();

    const filteredResults = [];

    for (const result of this.searchResults) {
      try {
        const currentValue = this._readValue(result.address, result.type, true);
        let match = false;

        switch (compareType) {
          case 'exact':
            match = this._compareValues(currentValue, value, result.type);
            break;
          case 'changed':
            match = !this._compareValues(currentValue, result.value, result.type);
            break;
          case 'unchanged':
            match = this._compareValues(currentValue, result.value, result.type);
            break;
          case 'increased':
            match = currentValue > result.value;
            break;
          case 'decreased':
            match = currentValue < result.value;
            break;
        }

        if (match) {
          filteredResults.push({
            address: result.address,
            value: currentValue,
            type: result.type,
            oldValue: result.value
          });
        }
      } catch (e) {
        // 忽略越界错误
      }
    }

    this.searchResults = filteredResults;
    console.timeEnd('Next Scan');
    console.log(`Filtered to ${this.searchResults.length} results`);
    return this.searchResults;
  }

  /**
   * 搜索变化的值
   */
  scanChanged() {
    return this.nextScan(null, 'changed');
  }

  /**
   * 搜索未变化的值
   */
  scanUnchanged() {
    return this.nextScan(null, 'unchanged');
  }

  /**
   * 搜索增加的值
   */
  scanIncreased() {
    return this.nextScan(null, 'increased');
  }

  /**
   * 搜索减少的值
   */
  scanDecreased() {
    return this.nextScan(null, 'decreased');
  }

  /**
   * 读取指定地址的值
   */
  readValue(address, type = 'int32', littleEndian = true) {
    this.updateMemoryView();
    return this._readValue(address, type, littleEndian);
  }

  /**
   * 写入指定地址的值
   */
  writeValue(address, value, type = 'int32', littleEndian = true) {
    this.updateMemoryView();

    switch (type) {
      case 'int8':
        this.dataView.setInt8(address, value);
        break;
      case 'uint8':
        this.dataView.setUint8(address, value);
        break;
      case 'int16':
        this.dataView.setInt16(address, value, littleEndian);
        break;
      case 'uint16':
        this.dataView.setUint16(address, value, littleEndian);
        break;
      case 'int32':
        this.dataView.setInt32(address, value, littleEndian);
        break;
      case 'uint32':
        this.dataView.setUint32(address, value, littleEndian);
        break;
      case 'float32':
        this.dataView.setFloat32(address, value, littleEndian);
        break;
      case 'float64':
        this.dataView.setFloat64(address, value, littleEndian);
        break;
    }

    console.log(`Wrote ${value} to address 0x${address.toString(16)}`);
  }

  /**
   * 监控地址变化
   */
  watchAddresses(addresses, interval = 1000) {
    const intervalId = setInterval(() => {
      this.updateMemoryView();
      console.log('\n--- Memory Watch ---');
      for (const addr of addresses) {
        const value = this._readValue(addr.address, addr.type, true);
        console.log(`0x${addr.address.toString(16).padStart(8, '0')}: ${value} (${addr.type})`);
      }
    }, interval);

    return intervalId;
  }

  /**
   * 停止监控
   */
  stopWatch(intervalId) {
    clearInterval(intervalId);
  }

  /**
   * 获取当前搜索结果
   */
  getResults(limit = 100) {
    return this.searchResults.slice(0, limit);
  }

  /**
   * 重置搜索
   */
  reset() {
    this.searchResults = [];
    console.log('Search results cleared');
  }

  /**
   * 内部方法：读取值
   */
  _readValue(address, type, littleEndian) {
    switch (type) {
      case 'int8':
        return this.dataView.getInt8(address);
      case 'uint8':
        return this.dataView.getUint8(address);
      case 'int16':
        return this.dataView.getInt16(address, littleEndian);
      case 'uint16':
        return this.dataView.getUint16(address, littleEndian);
      case 'int32':
        return this.dataView.getInt32(address, littleEndian);
      case 'uint32':
        return this.dataView.getUint32(address, littleEndian);
      case 'float32':
        return this.dataView.getFloat32(address, littleEndian);
      case 'float64':
        return this.dataView.getFloat64(address, littleEndian);
      default:
        throw new Error(`Unknown type: ${type}`);
    }
  }

  /**
   * 内部方法：比较值
   */
  _compareValues(val1, val2, type) {
    if (type.includes('float')) {
      // 浮点数使用容差比较
      const epsilon = type === 'float32' ? 0.0001 : 0.000001;
      return Math.abs(val1 - val2) < epsilon;
    }
    return val1 === val2;
  }

  /**
   * 内部方法：获取类型信息
   */
  _getTypeInfo(type) {
    const types = {
      'int8': { size: 1, signed: true },
      'uint8': { size: 1, signed: false },
      'int16': { size: 2, signed: true },
      'uint16': { size: 2, signed: false },
      'int32': { size: 4, signed: true },
      'uint32': { size: 4, signed: false },
      'float32': { size: 4, signed: true },
      'float64': { size: 8, signed: true }
    };
    return types[type] || types['int32'];
  }

  /**
   * 导出搜索结果到 JSON
   */
  exportResults(filename = 'scan_results.json') {
    const data = JSON.stringify(this.searchResults, null, 2);
    console.log(`Export ${this.searchResults.length} results:`);
    console.log(data);
    return data;
  }

  /**
   * 显示内存十六进制转储
   */
  hexDump(address, length = 128) {
    this.updateMemoryView();
    console.log(`\nMemory dump at 0x${address.toString(16)}:`);

    for (let i = 0; i < length; i += 16) {
      const offset = address + i;
      const hex = [];
      const ascii = [];

      for (let j = 0; j < 16 && offset + j < this.buffer.length; j++) {
        const byte = this.buffer[offset + j];
        hex.push(byte.toString(16).padStart(2, '0'));
        ascii.push(byte >= 32 && byte <= 126 ? String.fromCharCode(byte) : '.');
      }

      console.log(`0x${offset.toString(16).padStart(8, '0')}  ${hex.join(' ').padEnd(48, ' ')}  ${ascii.join('')}`);
    }
  }
}

// 使用示例
if (typeof module !== 'undefined' && module.exports) {
  module.exports = WasmMemoryScanner;
}
