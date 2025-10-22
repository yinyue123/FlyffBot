/**
 * Content Script
 * 运行在隔离的上下文中，作为页面和扩展之间的桥梁
 */

console.log('[WASM Scanner] Content script loaded');

let injected = false;
let scannerReady = false;

// 注入脚本到页面
function injectScript(file) {
  const script = document.createElement('script');
  script.src = chrome.runtime.getURL(file);
  script.onload = function() {
    this.remove();
    console.log(`[WASM Scanner] Injected ${file}`);
  };
  (document.head || document.documentElement).appendChild(script);
}

// 自动注入
function autoInject() {
  if (injected) return;

  try {
    // 注入扫描器库
    injectScript('content/memory-scanner.js');

    // 注入主脚本
    setTimeout(() => {
      injectScript('content/injected.js');
    }, 100);

    injected = true;
    console.log('[WASM Scanner] Scripts injected');
  } catch (e) {
    console.error('[WASM Scanner] Injection failed:', e);
  }
}

// 监听页面消息
window.addEventListener('message', (event) => {
  if (event.source !== window) return;
  if (!event.data.type) return;

  const { type, payload } = event.data;

  // 转发特定消息到 background 或 devtools
  switch (type) {
    case 'WASM_SCANNER_MEMORY_CAPTURED':
      console.log('[WASM Scanner] Memory captured, count:', payload.count);
      scannerReady = true;
      // 可以通知 popup 或 devtools
      chrome.runtime.sendMessage({
        action: 'memoryCaptured',
        count: payload.count
      }).catch(() => {});
      break;

    case 'WASM_SCANNER_STATUS':
    case 'WASM_SCANNER_LIST':
    case 'WASM_SCANNER_SCAN_RESULT':
    case 'WASM_SCANNER_READ_RESULT':
    case 'WASM_SCANNER_WRITE_RESULT':
    case 'WASM_SCANNER_RESULTS':
      // 存储最近的响应，供后续请求使用
      window.__lastScannerResponse__ = { type, payload };
      break;
  }
});

// 监听来自 popup 或 devtools 的消息
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  console.log('[WASM Scanner] Received message:', request);

  switch (request.action) {
    case 'inject':
      autoInject();
      sendResponse({ success: true });
      break;

    case 'checkStatus':
      if (!injected) {
        sendResponse({ connected: false, wasmCount: 0 });
      } else {
        // 请求状态
        window.postMessage({ type: 'WASM_SCANNER_GET_STATUS' }, '*');

        // 等待响应
        setTimeout(() => {
          const response = window.__lastScannerResponse__;
          if (response && response.type === 'WASM_SCANNER_STATUS') {
            sendResponse(response.payload);
          } else {
            sendResponse({ connected: scannerReady, wasmCount: 0 });
          }
        }, 100);

        return true; // 异步响应
      }
      break;

    case 'getWasmList':
      window.postMessage({ type: 'WASM_SCANNER_GET_LIST' }, '*');

      setTimeout(() => {
        const response = window.__lastScannerResponse__;
        if (response && response.type === 'WASM_SCANNER_LIST') {
          sendResponse({ success: true, list: response.payload });
        } else {
          sendResponse({ success: false, list: [] });
        }
      }, 100);

      return true; // 异步响应

    case 'selectWasm':
      window.postMessage({
        type: 'WASM_SCANNER_SELECT',
        payload: { index: request.index }
      }, '*');
      sendResponse({ success: true });
      break;

    case 'firstScan':
      window.postMessage({
        type: 'WASM_SCANNER_FIRST_SCAN',
        payload: {
          value: request.value,
          type: request.type
        }
      }, '*');

      setTimeout(() => {
        const response = window.__lastScannerResponse__;
        if (response && response.type === 'WASM_SCANNER_SCAN_RESULT') {
          sendResponse(response.payload);
        } else {
          sendResponse({ success: false, error: 'No response' });
        }
      }, 200);

      return true; // 异步响应

    case 'nextScan':
      window.postMessage({
        type: 'WASM_SCANNER_NEXT_SCAN',
        payload: {
          value: request.value,
          compareType: request.compareType || 'exact'
        }
      }, '*');

      setTimeout(() => {
        const response = window.__lastScannerResponse__;
        if (response && response.type === 'WASM_SCANNER_SCAN_RESULT') {
          sendResponse(response.payload);
        } else {
          sendResponse({ success: false, error: 'No response' });
        }
      }, 200);

      return true; // 异步响应

    case 'readValue':
      window.postMessage({
        type: 'WASM_SCANNER_READ_VALUE',
        payload: {
          address: request.address,
          type: request.type
        }
      }, '*');

      setTimeout(() => {
        const response = window.__lastScannerResponse__;
        if (response && response.type === 'WASM_SCANNER_READ_RESULT') {
          sendResponse(response.payload);
        } else {
          sendResponse({ success: false, error: 'No response' });
        }
      }, 100);

      return true; // 异步响应

    case 'writeValue':
      window.postMessage({
        type: 'WASM_SCANNER_WRITE_VALUE',
        payload: {
          address: request.address,
          value: request.value,
          type: request.type
        }
      }, '*');

      setTimeout(() => {
        const response = window.__lastScannerResponse__;
        if (response && response.type === 'WASM_SCANNER_WRITE_RESULT') {
          sendResponse(response.payload);
        } else {
          sendResponse({ success: false, error: 'No response' });
        }
      }, 100);

      return true; // 异步响应

    case 'getResults':
      window.postMessage({
        type: 'WASM_SCANNER_GET_RESULTS',
        payload: { limit: request.limit || 1000 }
      }, '*');

      setTimeout(() => {
        const response = window.__lastScannerResponse__;
        if (response && response.type === 'WASM_SCANNER_RESULTS') {
          sendResponse({ success: true, results: response.payload });
        } else {
          sendResponse({ success: false, results: [] });
        }
      }, 100);

      return true; // 异步响应

    case 'manualScan':
      // 手动触发扫描
      window.postMessage({ type: 'WASM_SCANNER_MANUAL_SCAN' }, '*');

      setTimeout(() => {
        const response = window.__lastScannerResponse__;
        if (response && response.type === 'WASM_SCANNER_STATUS') {
          sendResponse(response.payload);
        } else {
          sendResponse({ connected: scannerReady, wasmCount: 0 });
        }
      }, 500);

      return true; // 异步响应

    case 'getScanStats':
      // 获取当前扫描结果统计
      window.postMessage({ type: 'WASM_SCANNER_GET_STATS' }, '*');

      setTimeout(() => {
        const response = window.__lastScannerResponse__;
        if (response && response.type === 'WASM_SCANNER_STATS') {
          sendResponse(response.payload);
        } else {
          sendResponse({ success: false, count: 0 });
        }
      }, 100);

      return true; // 异步响应

    default:
      sendResponse({ success: false, error: 'Unknown action' });
  }
});

// 不自动注入，等待用户手动触发或页面完全加载后再注入
// 这样可以避免影响游戏的初始化
if (document.readyState === 'complete') {
  // 如果页面已完全加载，延迟 3 秒后自动注入
  setTimeout(autoInject, 3000);
} else {
  // 否则等待页面加载完成
  window.addEventListener('load', () => {
    setTimeout(autoInject, 3000);
  });
}

console.log('[WASM Scanner] Content script ready');
