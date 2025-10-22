// Popup 脚本
let currentTab = null;

// 初始化
document.addEventListener('DOMContentLoaded', async () => {
  // 获取当前标签页
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  currentTab = tab;

  // 检查连接状态
  checkConnectionStatus();

  // 绑定事件
  document.getElementById('injectBtn').addEventListener('click', injectScanner);
  document.getElementById('openDevToolsBtn').addEventListener('click', openDevTools);
  document.getElementById('refreshBtn').addEventListener('click', refreshWasmList);
  document.getElementById('firstScanBtn').addEventListener('click', performFirstScan);
  document.getElementById('nextScanBtn').addEventListener('click', performNextScan);
  document.getElementById('helpLink').addEventListener('click', showHelp);

  // 刷新 WASM 列表
  refreshWasmList();

  // 加载上次的扫描结果
  loadScanResults();
});

// 检查连接状态
async function checkConnectionStatus() {
  try {
    const response = await chrome.tabs.sendMessage(currentTab.id, {
      action: 'checkStatus'
    });

    updateStatus(response.connected, response.wasmCount || 0);
  } catch (e) {
    updateStatus(false, 0);
  }
}

// 更新状态显示
function updateStatus(connected, wasmCount) {
  const dot = document.getElementById('statusDot');
  const text = document.getElementById('statusText');

  if (connected) {
    dot.classList.add('connected');
    text.textContent = `已连接 (${wasmCount} 个实例)`;
  } else {
    dot.classList.remove('connected');
    text.textContent = '未连接';
  }
}

// 注入扫描器
async function injectScanner() {
  const btn = document.getElementById('injectBtn');
  btn.disabled = true;
  btn.textContent = '注入中...';

  try {
    // 发送注入消息
    const response = await chrome.tabs.sendMessage(currentTab.id, {
      action: 'inject'
    });

    if (response.success) {
      showNotification('✅ 扫描器注入成功！', 'success');
      setTimeout(() => {
        checkConnectionStatus();
        refreshWasmList();
      }, 500);
    } else {
      showNotification('❌ 注入失败: ' + response.error, 'error');
    }
  } catch (e) {
    showNotification('❌ 注入失败: ' + e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = '💉 注入扫描器';
  }
}

// 打开开发者工具
function openDevTools() {
  showNotification('请按 F12 打开开发者工具，然后切换到 "WASM Scanner" 标签页', 'info');
}

// 刷新 WASM 列表
async function refreshWasmList() {
  const listEl = document.getElementById('wasmList');
  listEl.innerHTML = '<div class="loading">检测中...</div>';

  try {
    // 先触发手动扫描
    await chrome.tabs.sendMessage(currentTab.id, {
      action: 'manualScan'
    });

    // 然后获取列表
    const response = await chrome.tabs.sendMessage(currentTab.id, {
      action: 'getWasmList'
    });

    if (response.success && response.list && response.list.length > 0) {
      listEl.innerHTML = '';
      response.list.forEach((wasm, index) => {
        const item = document.createElement('div');
        item.className = 'wasm-item';
        if (index === 0) item.classList.add('selected');

        const sizeMB = (wasm.size / 1024 / 1024).toFixed(2);
        item.innerHTML = `
          <div class="wasm-name">Memory #${index}</div>
          <div class="wasm-size">${sizeMB} MB</div>
        `;

        item.addEventListener('click', () => {
          document.querySelectorAll('.wasm-item').forEach(el =>
            el.classList.remove('selected')
          );
          item.classList.add('selected');
          selectWasmInstance(index);
        });

        listEl.appendChild(item);
      });

      // 更新状态
      updateStatus(true, response.list.length);
    } else {
      listEl.innerHTML = '<div class="loading">未发现 WASM 实例<br><small>请先注入扫描器</small></div>';
      updateStatus(false, 0);
    }
  } catch (e) {
    listEl.innerHTML = '<div class="loading">未连接<br><small>请先注入扫描器</small></div>';
    updateStatus(false, 0);
  }
}

// 选择 WASM 实例
async function selectWasmInstance(index) {
  try {
    await chrome.tabs.sendMessage(currentTab.id, {
      action: 'selectWasm',
      index: index
    });
    showNotification(`已选择 Memory #${index}`, 'info');
  } catch (e) {
    console.error('Failed to select WASM:', e);
  }
}

// 执行首次扫描
async function performFirstScan() {
  const value = parseFloat(document.getElementById('quickValue').value);
  const type = document.getElementById('quickType').value;

  if (isNaN(value)) {
    showNotification('请输入有效的数值', 'error');
    return;
  }

  const btn = document.getElementById('firstScanBtn');
  btn.disabled = true;
  btn.textContent = '扫描中...';

  try {
    const response = await chrome.tabs.sendMessage(currentTab.id, {
      action: 'firstScan',
      value: value,
      type: type
    });

    if (response.success) {
      updateScanResult(response.count);
      showNotification(`找到 ${response.count} 个结果`, 'success');

      // 如果结果太多，提示用户继续扫描
      if (response.count > 1000) {
        setTimeout(() => {
          showNotification(`结果过多 (${response.count})，建议继续扫描缩小范围`, 'info');
        }, 2000);
      }
    } else {
      showNotification('扫描失败: ' + response.error, 'error');
    }
  } catch (e) {
    showNotification('扫描失败: ' + e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = '首次扫描';
  }
}

// 执行继续扫描
async function performNextScan() {
  const value = parseFloat(document.getElementById('quickValue').value);

  if (isNaN(value)) {
    showNotification('请输入有效的数值', 'error');
    return;
  }

  const btn = document.getElementById('nextScanBtn');
  btn.disabled = true;
  btn.textContent = '扫描中...';

  try {
    const response = await chrome.tabs.sendMessage(currentTab.id, {
      action: 'nextScan',
      value: value
    });

    if (response.success) {
      updateScanResult(response.count);
      showNotification(`剩余 ${response.count} 个结果`, 'success');

      // 如果结果很少了，提示用户可能找到了
      if (response.count > 0 && response.count <= 10) {
        setTimeout(() => {
          showNotification(`✓ 结果已缩小到 ${response.count} 个地址！`, 'success');
        }, 2000);
      } else if (response.count === 0) {
        showNotification('没有找到匹配的结果，请重新开始扫描', 'info');
      }
    } else {
      showNotification('扫描失败: ' + response.error, 'error');
    }
  } catch (e) {
    showNotification('扫描失败: ' + e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = '继续扫描';
  }
}

// 加载扫描结果
async function loadScanResults() {
  try {
    // 请求当前的扫描结果统计
    const response = await chrome.tabs.sendMessage(currentTab.id, {
      action: 'getScanStats'
    });

    if (response && response.success) {
      updateScanResult(response.count);
    }
  } catch (e) {
    // 如果无法获取，保持默认显示
    console.log('Could not load scan results:', e.message);
  }
}

// 更新扫描结果显示
function updateScanResult(count) {
  const resultEl = document.getElementById('scanResult');
  resultEl.innerHTML = `<span class="result-count">结果: <strong>${count}</strong></span>`;
}

// 显示帮助
function showHelp(e) {
  e.preventDefault();
  chrome.tabs.create({
    url: chrome.runtime.getURL('help.html')
  });
}

// 显示通知
function showNotification(message, type = 'info') {
  // 简单的通知实现
  const resultEl = document.getElementById('scanResult');
  const colors = {
    success: '#4ec9b0',
    error: '#f48771',
    info: '#569cd6'
  };

  resultEl.style.color = colors[type];
  resultEl.textContent = message;

  setTimeout(() => {
    resultEl.style.color = '';
  }, 3000);
}
