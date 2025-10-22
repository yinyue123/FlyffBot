/**
 * 注入到页面的脚本
 * 运行在页面上下文中，可以访问页面的 WASM 实例
 */

(function() {
  'use strict';

  console.log('[WASM Scanner] Injected script loaded');

  // 全局状态
  window.__wasmScanner__ = {
    memories: [],
    scanners: [],
    currentScanner: null,
    intercepted: false
  };

  // 拦截 WebAssembly.instantiate 来捕获所有 WASM 实例
  function interceptWebAssembly() {
    if (window.__wasmScanner__.intercepted) return;

    const originalInstantiate = WebAssembly.instantiate;
    const originalInstantiateStreaming = WebAssembly.instantiateStreaming;

    // 拦截 instantiate
    WebAssembly.instantiate = async function(...args) {
      console.log('[WASM Scanner] Intercepting WebAssembly.instantiate');
      const result = await originalInstantiate.apply(this, args);

      if (result.instance && result.instance.exports.memory) {
        captureMemory(result.instance.exports.memory, 'instantiate');
      }

      return result;
    };

    // 拦截 instantiateStreaming
    WebAssembly.instantiateStreaming = async function(...args) {
      console.log('[WASM Scanner] Intercepting WebAssembly.instantiateStreaming');
      const result = await originalInstantiateStreaming.apply(this, args);

      if (result.instance && result.instance.exports.memory) {
        captureMemory(result.instance.exports.memory, 'instantiateStreaming');
      }

      return result;
    };

    window.__wasmScanner__.intercepted = true;
    console.log('[WASM Scanner] WebAssembly interception enabled');
  }

  // 捕获 Memory 实例
  function captureMemory(memory, source) {
    if (!(memory instanceof WebAssembly.Memory)) return;

    // 检查是否已存在
    const exists = window.__wasmScanner__.memories.some(m => m.memory === memory);
    if (exists) return;

    console.log(`[WASM Scanner] Captured WASM Memory from ${source}`);

    window.__wasmScanner__.memories.push({
      memory: memory,
      source: source,
      capturedAt: Date.now()
    });

    // 创建扫描器
    if (typeof WasmMemoryScanner !== 'undefined') {
      const scanner = new WasmMemoryScanner(memory);
      window.__wasmScanner__.scanners.push(scanner);

      if (!window.__wasmScanner__.currentScanner) {
        window.__wasmScanner__.currentScanner = scanner;
      }

      console.log(`[WASM Scanner] Created scanner, total: ${window.__wasmScanner__.scanners.length}`);
    }

    // 通知内容脚本
    window.postMessage({
      type: 'WASM_SCANNER_MEMORY_CAPTURED',
      count: window.__wasmScanner__.memories.length
    }, '*');
  }

  // 扫描现有的 WASM 实例 - 优化版本
  function scanExistingWasm() {
    console.log('[WASM Scanner] Scanning for existing WASM instances...');

    // 方法 1: 检查 Unity WebGL 的常见位置
    const unityLocations = [
      'unityInstance',
      'gameInstance',
      'Module'
    ];

    for (const key of unityLocations) {
      try {
        if (window[key]) {
          console.log(`[WASM Scanner] Found ${key}`);

          // 检查 Module.HEAPU8.buffer (Unity/Emscripten 特有)
          if (window[key].Module && window[key].Module.HEAPU8) {
            console.log('[WASM Scanner] Found HEAPU8 in Module');
            // 从 buffer 创建 Memory 包装器
            const buffer = window[key].Module.HEAPU8.buffer;
            if (buffer instanceof SharedArrayBuffer || buffer instanceof ArrayBuffer) {
              console.log(`[WASM Scanner] Found buffer: ${buffer.byteLength} bytes`);
              // 尝试查找对应的 Memory 对象
              tryFindMemoryFromBuffer(buffer, `${key}.Module`);
            }
          }

          // 检查是否直接有 memory 属性
          if (window[key].memory instanceof WebAssembly.Memory) {
            captureMemory(window[key].memory, `global.${key}.memory`);
          }
        }
      } catch (e) {
        console.log(`[WASM Scanner] Error checking ${key}:`, e.message);
      }
    }

    // 方法 2: 直接检查 Module (Emscripten)
    if (typeof Module !== 'undefined') {
      console.log('[WASM Scanner] Found global Module');

      if (Module.HEAPU8 && Module.HEAPU8.buffer) {
        console.log(`[WASM Scanner] Found Module.HEAPU8.buffer: ${Module.HEAPU8.buffer.byteLength} bytes`);
        tryFindMemoryFromBuffer(Module.HEAPU8.buffer, 'Module');
      }

      // 检查 asm.js 导出
      if (Module.asm && Module.asm.memory instanceof WebAssembly.Memory) {
        captureMemory(Module.asm.memory, 'Module.asm.memory');
      }

      // 检查 wasmMemory
      if (Module.wasmMemory instanceof WebAssembly.Memory) {
        captureMemory(Module.wasmMemory, 'Module.wasmMemory');
      }
    }

    // 方法 3: 检查其他常见位置
    const otherLocations = ['wasmMemory', 'WasmInstance', '_wasmInstance', 'instance', 'exports'];
    for (const key of otherLocations) {
      try {
        if (window[key] instanceof WebAssembly.Memory) {
          captureMemory(window[key], `global.${key}`);
        }
      } catch (e) {
        // 忽略
      }
    }

    console.log(`[WASM Scanner] Scan complete. Found ${window.__wasmScanner__.memories.length} WASM instances`);
  }

  // 尝试从 ArrayBuffer 找到对应的 WebAssembly.Memory
  function tryFindMemoryFromBuffer(buffer, source) {
    // 如果已经捕获过这个 buffer，跳过
    const exists = window.__wasmScanner__.memories.some(m => m.memory.buffer === buffer);
    if (exists) {
      console.log(`[WASM Scanner] Buffer from ${source} already captured`);
      return;
    }

    // 创建一个伪 Memory 对象来包装这个 buffer
    // 注意：这不是真正的 WebAssembly.Memory，但可以用于扫描
    const fakeMemory = {
      buffer: buffer,
      grow: function() { throw new Error('Cannot grow fake memory'); }
    };

    console.log(`[WASM Scanner] Creating scanner for buffer from ${source}`);

    window.__wasmScanner__.memories.push({
      memory: fakeMemory,
      source: source,
      capturedAt: Date.now(),
      isFake: true
    });

    // 创建扫描器
    if (typeof WasmMemoryScanner !== 'undefined') {
      try {
        const scanner = new WasmMemoryScanner(fakeMemory);
        window.__wasmScanner__.scanners.push(scanner);

        if (!window.__wasmScanner__.currentScanner) {
          window.__wasmScanner__.currentScanner = scanner;
        }

        console.log(`[WASM Scanner] Created scanner for ${source}, total: ${window.__wasmScanner__.scanners.length}`);

        // 通知成功
        window.postMessage({
          type: 'WASM_SCANNER_MEMORY_CAPTURED',
          payload: { count: window.__wasmScanner__.memories.length }
        }, '*');
      } catch (e) {
        console.error(`[WASM Scanner] Failed to create scanner for ${source}:`, e);
      }
    }
  }

  // 消息处理
  window.addEventListener('message', (event) => {
    if (event.source !== window) return;
    if (!event.data.type) return;

    const { type, payload } = event.data;

    switch (type) {
      case 'WASM_SCANNER_GET_STATUS':
        window.postMessage({
          type: 'WASM_SCANNER_STATUS',
          payload: {
            connected: window.__wasmScanner__.memories.length > 0,
            wasmCount: window.__wasmScanner__.memories.length,
            scannersCount: window.__wasmScanner__.scanners.length
          }
        }, '*');
        break;

      case 'WASM_SCANNER_GET_LIST':
        const list = window.__wasmScanner__.memories.map((m, i) => ({
          index: i,
          source: m.source,
          size: m.memory.buffer.byteLength,
          capturedAt: m.capturedAt
        }));

        window.postMessage({
          type: 'WASM_SCANNER_LIST',
          payload: list
        }, '*');
        break;

      case 'WASM_SCANNER_SELECT':
        const index = payload.index;
        if (index >= 0 && index < window.__wasmScanner__.scanners.length) {
          window.__wasmScanner__.currentScanner = window.__wasmScanner__.scanners[index];
          console.log(`[WASM Scanner] Selected scanner #${index}`);
        }
        break;

      case 'WASM_SCANNER_FIRST_SCAN':
        if (window.__wasmScanner__.currentScanner) {
          try {
            const results = window.__wasmScanner__.currentScanner.firstScan(
              payload.value,
              payload.type
            );
            window.postMessage({
              type: 'WASM_SCANNER_SCAN_RESULT',
              payload: {
                success: true,
                count: results.length,
                results: results.slice(0, 1000) // 最多返回 1000 个
              }
            }, '*');
          } catch (e) {
            window.postMessage({
              type: 'WASM_SCANNER_SCAN_RESULT',
              payload: {
                success: false,
                error: e.message
              }
            }, '*');
          }
        }
        break;

      case 'WASM_SCANNER_NEXT_SCAN':
        if (window.__wasmScanner__.currentScanner) {
          try {
            const results = window.__wasmScanner__.currentScanner.nextScan(
              payload.value,
              payload.compareType || 'exact'
            );
            window.postMessage({
              type: 'WASM_SCANNER_SCAN_RESULT',
              payload: {
                success: true,
                count: results.length,
                results: results.slice(0, 1000)
              }
            }, '*');
          } catch (e) {
            window.postMessage({
              type: 'WASM_SCANNER_SCAN_RESULT',
              payload: {
                success: false,
                error: e.message
              }
            }, '*');
          }
        }
        break;

      case 'WASM_SCANNER_READ_VALUE':
        if (window.__wasmScanner__.currentScanner) {
          try {
            const value = window.__wasmScanner__.currentScanner.readValue(
              payload.address,
              payload.type
            );
            window.postMessage({
              type: 'WASM_SCANNER_READ_RESULT',
              payload: { success: true, value: value }
            }, '*');
          } catch (e) {
            window.postMessage({
              type: 'WASM_SCANNER_READ_RESULT',
              payload: { success: false, error: e.message }
            }, '*');
          }
        }
        break;

      case 'WASM_SCANNER_WRITE_VALUE':
        if (window.__wasmScanner__.currentScanner) {
          try {
            window.__wasmScanner__.currentScanner.writeValue(
              payload.address,
              payload.value,
              payload.type
            );
            window.postMessage({
              type: 'WASM_SCANNER_WRITE_RESULT',
              payload: { success: true }
            }, '*');
          } catch (e) {
            window.postMessage({
              type: 'WASM_SCANNER_WRITE_RESULT',
              payload: { success: false, error: e.message }
            }, '*');
          }
        }
        break;

      case 'WASM_SCANNER_GET_RESULTS':
        if (window.__wasmScanner__.currentScanner) {
          const results = window.__wasmScanner__.currentScanner.getResults(
            payload.limit || 1000
          );
          window.postMessage({
            type: 'WASM_SCANNER_RESULTS',
            payload: results
          }, '*');
        }
        break;

      case 'WASM_SCANNER_MANUAL_SCAN':
        // 手动扫描
        console.log('[WASM Scanner] Manual scan triggered');
        scanExistingWasm();

        // 立即返回状态
        setTimeout(() => {
          window.postMessage({
            type: 'WASM_SCANNER_STATUS',
            payload: {
              connected: window.__wasmScanner__.memories.length > 0,
              wasmCount: window.__wasmScanner__.memories.length,
              scannersCount: window.__wasmScanner__.scanners.length
            }
          }, '*');
        }, 100);
        break;

      case 'WASM_SCANNER_GET_STATS':
        // 获取扫描统计信息
        let count = 0;
        let lastScanType = 'none';
        if (window.__wasmScanner__.currentScanner) {
          if (window.__wasmScanner__.currentScanner.searchResults) {
            count = window.__wasmScanner__.currentScanner.searchResults.length;
            lastScanType = count > 0 ? 'has_results' : 'no_results';
          }
        }

        window.postMessage({
          type: 'WASM_SCANNER_STATS',
          payload: {
            success: true,
            count: count,
            hasScanner: !!window.__wasmScanner__.currentScanner,
            memoryCount: window.__wasmScanner__.memories.length,
            lastScanType: lastScanType
          }
        }, '*');
        break;
    }
  });

  // 初始化
  function init() {
    // 拦截 WebAssembly
    interceptWebAssembly();

    // 延迟扫描，让游戏有时间初始化
    // 多次扫描以确保捕获到 WASM 实例
    setTimeout(scanExistingWasm, 2000);
    setTimeout(scanExistingWasm, 5000);
    setTimeout(scanExistingWasm, 10000);

    console.log('[WASM Scanner] Initialization complete');
  }

  // 如果 DOM 已加载则立即初始化，否则等待
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  // 暴露调试接口
  window.__wasmScannerDebug__ = {
    getMemories: () => window.__wasmScanner__.memories,
    getScanners: () => window.__wasmScanner__.scanners,
    getCurrentScanner: () => window.__wasmScanner__.currentScanner,
    scan: scanExistingWasm
  };

  console.log('[WASM Scanner] Debug interface available at window.__wasmScannerDebug__');
})();
