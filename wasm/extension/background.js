/**
 * Background Service Worker
 * 处理扩展的后台任务
 */

console.log('[WASM Scanner] Background service worker loaded');

// 扩展安装时
chrome.runtime.onInstalled.addListener((details) => {
  if (details.reason === 'install') {
    console.log('[WASM Scanner] Extension installed');
    // 可以打开欢迎页面
    // chrome.tabs.create({ url: 'welcome.html' });
  } else if (details.reason === 'update') {
    console.log('[WASM Scanner] Extension updated');
  }
});

// 监听来自 content script 的消息
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  console.log('[WASM Scanner] Background received message:', request);

  switch (request.action) {
    case 'memoryCaptured':
      console.log(`[WASM Scanner] Memory captured in tab ${sender.tab?.id}, count: ${request.count}`);
      // 可以更新 badge
      if (sender.tab?.id) {
        chrome.action.setBadgeText({
          tabId: sender.tab.id,
          text: request.count.toString()
        });
        chrome.action.setBadgeBackgroundColor({
          tabId: sender.tab.id,
          color: '#4ec9b0'
        });
      }
      break;

    case 'log':
      console.log(`[WASM Scanner] ${request.level}:`, request.message);
      break;

    default:
      console.log('[WASM Scanner] Unknown action:', request.action);
  }

  sendResponse({ received: true });
});

// 监听标签页更新
chrome.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  if (changeInfo.status === 'complete') {
    // 重置 badge
    chrome.action.setBadgeText({ tabId: tabId, text: '' });
  }
});

// 监听标签页切换
chrome.tabs.onActivated.addListener((activeInfo) => {
  console.log('[WASM Scanner] Tab activated:', activeInfo.tabId);
});

// 提供一个测试命令
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'ping') {
    sendResponse({ pong: true });
  }
});
