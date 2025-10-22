/**
 * DevTools 初始化脚本
 * 创建一个新的面板
 */

console.log('[WASM Scanner] DevTools script loaded');

// 创建面板
chrome.devtools.panels.create(
  'WASM Scanner',
  'icons/icon48.png',
  'devtools/panel.html',
  (panel) => {
    console.log('[WASM Scanner] DevTools panel created');
  }
);
