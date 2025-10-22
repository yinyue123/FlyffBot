/**
 * DevTools Panel 主脚本
 */

let currentResults = [];
let watchList = [];
let autoRefreshInterval = null;

// 初始化
document.addEventListener('DOMContentLoaded', () => {
  log('WASM Memory Scanner 已加载', 'info');

  // 绑定事件
  document.getElementById('refreshWasmBtn').addEventListener('click', refreshWasmList);
  document.getElementById('firstScanBtn').addEventListener('click', performFirstScan);
  document.getElementById('nextScanBtn').addEventListener('click', performNextScan);
  document.getElementById('resetScanBtn').addEventListener('click', resetScan);
  document.getElementById('modifyBtn').addEventListener('click', modifyMemory);
  document.getElementById('exportBtn').addEventListener('click', exportResults);
  document.getElementById('clearWatchBtn').addEventListener('click', clearWatchList);
  document.getElementById('clearLogBtn').addEventListener('click', clearLog);
  document.getElementById('filterInput').addEventListener('input', filterResults);
  document.getElementById('autoRefresh').addEventListener('change', toggleAutoRefresh);

  // 初始化
  checkConnection();
  refreshWasmList();
  toggleAutoRefresh();
});

// 发送消息到 content script
function sendMessage(action, data = {}) {
  return new Promise((resolve, reject) => {
    chrome.devtools.inspectedWindow.eval(
      `chrome.runtime.sendMessage(${JSON.stringify({ action, ...data })})`,
      (result, exceptionInfo) => {
        if (exceptionInfo) {
          reject(new Error(exceptionInfo.value));
        } else {
          resolve(result);
        }
      }
    );
  });
}

// 在页面中执行代码
function evalInPage(code) {
  return new Promise((resolve, reject) => {
    chrome.devtools.inspectedWindow.eval(code, (result, exceptionInfo) => {
      if (exceptionInfo) {
        reject(new Error(exceptionInfo.value));
      } else {
        resolve(result);
      }
    });
  });
}

// 检查连接状态
async function checkConnection() {
  try {
    const result = await evalInPage(`
      (function() {
        if (window.__wasmScanner__) {
          return {
            connected: true,
            wasmCount: window.__wasmScanner__.memories.length,
            scannersCount: window.__wasmScanner__.scanners.length
          };
        }
        return { connected: false, wasmCount: 0 };
      })()
    `);

    updateStatus(result.connected, result.wasmCount);
  } catch (e) {
    updateStatus(false, 0);
  }
}

// 更新状态
function updateStatus(connected, wasmCount) {
  const indicator = document.getElementById('statusIndicator');
  const text = document.getElementById('statusText');

  if (connected) {
    indicator.classList.add('connected');
    text.textContent = `已连接 (${wasmCount} 个 WASM 实例)`;
  } else {
    indicator.classList.remove('connected');
    text.textContent = '未连接 - 等待页面加载...';
  }
}

// 刷新 WASM 列表
async function refreshWasmList() {
  try {
    const memories = await evalInPage(`
      (function() {
        if (!window.__wasmScanner__) return [];
        return window.__wasmScanner__.memories.map((m, i) => ({
          index: i,
          source: m.source,
          size: m.memory.buffer.byteLength,
          capturedAt: m.capturedAt
        }));
      })()
    `);

    const listEl = document.getElementById('wasmList');
    listEl.innerHTML = '';

    if (memories && memories.length > 0) {
      memories.forEach((mem, index) => {
        const item = document.createElement('div');
        item.className = 'wasm-item' + (index === 0 ? ' selected' : '');

        const sizeMB = (mem.size / 1024 / 1024).toFixed(2);
        item.innerHTML = `
          <div class="wasm-name">Memory #${index}</div>
          <div class="wasm-info">
            来源: ${mem.source}<br>
            大小: ${sizeMB} MB
          </div>
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

      updateStatus(true, memories.length);
      log(`发现 ${memories.length} 个 WASM 实例`, 'success');
    } else {
      listEl.innerHTML = '<div class="loading">未发现 WASM 实例</div>';
      log('未发现 WASM 实例，请等待页面加载', 'warning');
    }
  } catch (e) {
    log(`刷新 WASM 列表失败: ${e.message}`, 'error');
  }
}

// 选择 WASM 实例
async function selectWasmInstance(index) {
  try {
    await evalInPage(`
      (function() {
        if (window.__wasmScanner__ && window.__wasmScanner__.scanners[${index}]) {
          window.__wasmScanner__.currentScanner = window.__wasmScanner__.scanners[${index}];
          return true;
        }
        return false;
      })()
    `);

    log(`已选择 Memory #${index}`, 'info');
  } catch (e) {
    log(`选择 WASM 实例失败: ${e.message}`, 'error');
  }
}

// 首次扫描
async function performFirstScan() {
  const value = parseFloat(document.getElementById('scanValue').value);
  const type = document.getElementById('scanType').value;

  if (isNaN(value)) {
    log('请输入有效的数值', 'error');
    return;
  }

  const btn = document.getElementById('firstScanBtn');
  btn.disabled = true;
  btn.textContent = '扫描中...';
  log(`开始首次扫描: 值=${value}, 类型=${type}`, 'info');

  const startTime = performance.now();

  try {
    const result = await evalInPage(`
      (function() {
        if (!window.__wasmScanner__ || !window.__wasmScanner__.currentScanner) {
          throw new Error('未找到扫描器');
        }
        const scanner = window.__wasmScanner__.currentScanner;
        const results = scanner.firstScan(${value}, '${type}');
        return {
          success: true,
          count: results.length,
          results: results.slice(0, 1000)
        };
      })()
    `);

    const endTime = performance.now();
    const scanTime = ((endTime - startTime) / 1000).toFixed(2);

    document.getElementById('scanTime').textContent = scanTime + 's';
    document.getElementById('resultCount').textContent = result.count;

    currentResults = result.results;
    displayResults(result.results);

    log(`扫描完成: 找到 ${result.count} 个结果，耗时 ${scanTime}s`, 'success');
  } catch (e) {
    log(`扫描失败: ${e.message}`, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = '🔍 首次扫描';
  }
}

// 继续扫描
async function performNextScan() {
  const value = parseFloat(document.getElementById('scanValue').value);
  const compareType = document.getElementById('compareType').value;

  if (compareType === 'exact' && isNaN(value)) {
    log('精确值模式需要输入数值', 'error');
    return;
  }

  const btn = document.getElementById('nextScanBtn');
  btn.disabled = true;
  btn.textContent = '扫描中...';
  log(`继续扫描: 比较类型=${compareType}`, 'info');

  const startTime = performance.now();

  try {
    const result = await evalInPage(`
      (function() {
        if (!window.__wasmScanner__ || !window.__wasmScanner__.currentScanner) {
          throw new Error('未找到扫描器');
        }
        const scanner = window.__wasmScanner__.currentScanner;
        const results = scanner.nextScan(${value || 0}, '${compareType}');
        return {
          success: true,
          count: results.length,
          results: results.slice(0, 1000)
        };
      })()
    `);

    const endTime = performance.now();
    const scanTime = ((endTime - startTime) / 1000).toFixed(2);

    document.getElementById('scanTime').textContent = scanTime + 's';
    document.getElementById('resultCount').textContent = result.count;

    currentResults = result.results;
    displayResults(result.results);

    log(`扫描完成: 剩余 ${result.count} 个结果，耗时 ${scanTime}s`, 'success');
  } catch (e) {
    log(`扫描失败: ${e.message}`, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = '🔄 继续扫描';
  }
}

// 重置扫描
async function resetScan() {
  try {
    await evalInPage(`
      (function() {
        if (window.__wasmScanner__ && window.__wasmScanner__.currentScanner) {
          window.__wasmScanner__.currentScanner.reset();
        }
      })()
    `);

    currentResults = [];
    document.getElementById('resultCount').textContent = '0';
    document.getElementById('scanTime').textContent = '-';
    document.getElementById('resultsBody').innerHTML = '<tr><td colspan="4" class="empty-state">等待扫描...</td></tr>';

    log('已重置扫描', 'info');
  } catch (e) {
    log(`重置失败: ${e.message}`, 'error');
  }
}

// 显示结果
function displayResults(results) {
  const tbody = document.getElementById('resultsBody');
  tbody.innerHTML = '';

  if (!results || results.length === 0) {
    tbody.innerHTML = '<tr><td colspan="4" class="empty-state">无结果</td></tr>';
    return;
  }

  results.forEach(result => {
    const tr = document.createElement('tr');
    tr.innerHTML = `
      <td class="address-cell">0x${result.address.toString(16).padStart(8, '0')}</td>
      <td class="value-cell">${result.value}</td>
      <td class="type-cell">${result.type}</td>
      <td class="actions-cell">
        <button class="btn action-btn btn-primary" onclick="addToWatch(${result.address}, '${result.type}')">➕ 监控</button>
        <button class="btn action-btn btn-secondary" onclick="quickModify(${result.address}, '${result.type}')">✏️ 修改</button>
      </td>
    `;
    tbody.appendChild(tr);
  });
}

// 添加到监控列表
window.addToWatch = function(address, type) {
  const existing = watchList.find(item => item.address === address);
  if (existing) {
    log('该地址已在监控列表中', 'warning');
    return;
  }

  watchList.push({ address, type, value: 0 });
  updateWatchList();
  log(`已添加 0x${address.toString(16)} 到监控列表`, 'success');
};

// 快速修改
window.quickModify = function(address, type) {
  document.getElementById('modifyAddress').value = '0x' + address.toString(16);
  document.getElementById('modifyType').value = type;
  document.getElementById('modifyValue').focus();
};

// 修改内存
async function modifyMemory() {
  const addressStr = document.getElementById('modifyAddress').value;
  const value = parseFloat(document.getElementById('modifyValue').value);
  const type = document.getElementById('modifyType').value;

  if (!addressStr || isNaN(value)) {
    log('请输入有效的地址和值', 'error');
    return;
  }

  const address = addressStr.startsWith('0x')
    ? parseInt(addressStr, 16)
    : parseInt(addressStr, 10);

  try {
    await evalInPage(`
      (function() {
        if (!window.__wasmScanner__ || !window.__wasmScanner__.currentScanner) {
          throw new Error('未找到扫描器');
        }
        window.__wasmScanner__.currentScanner.writeValue(${address}, ${value}, '${type}');
      })()
    `);

    log(`成功修改 0x${address.toString(16)} = ${value} (${type})`, 'success');
  } catch (e) {
    log(`修改失败: ${e.message}`, 'error');
  }
}

// 更新监控列表
async function updateWatchList() {
  const tbody = document.getElementById('watchBody');
  tbody.innerHTML = '';

  if (watchList.length === 0) {
    tbody.innerHTML = '<tr><td colspan="4" class="empty-state">监控列表为空</td></tr>';
    return;
  }

  for (let i = 0; i < watchList.length; i++) {
    const item = watchList[i];

    try {
      const value = await evalInPage(`
        (function() {
          if (window.__wasmScanner__ && window.__wasmScanner__.currentScanner) {
            return window.__wasmScanner__.currentScanner.readValue(${item.address}, '${item.type}');
          }
          return 'N/A';
        })()
      `);

      const tr = document.createElement('tr');
      const valueChanged = item.value !== value;

      tr.innerHTML = `
        <td class="address-cell">0x${item.address.toString(16).padStart(8, '0')}</td>
        <td class="value-cell ${valueChanged ? 'value-changed' : ''}">${value}</td>
        <td class="type-cell">${item.type}</td>
        <td class="actions-cell">
          <button class="btn action-btn btn-danger" onclick="removeFromWatch(${i})">🗑️</button>
        </td>
      `;

      tbody.appendChild(tr);
      item.value = value;
    } catch (e) {
      // 忽略错误
    }
  }
}

// 从监控列表移除
window.removeFromWatch = function(index) {
  const item = watchList[index];
  watchList.splice(index, 1);
  updateWatchList();
  log(`已移除 0x${item.address.toString(16)} 从监控列表`, 'info');
};

// 清空监控列表
function clearWatchList() {
  watchList = [];
  updateWatchList();
  log('已清空监控列表', 'info');
}

// 切换自动刷新
function toggleAutoRefresh() {
  const enabled = document.getElementById('autoRefresh').checked;

  if (autoRefreshInterval) {
    clearInterval(autoRefreshInterval);
    autoRefreshInterval = null;
  }

  if (enabled) {
    autoRefreshInterval = setInterval(() => {
      if (watchList.length > 0) {
        updateWatchList();
      }
    }, 1000);
    log('已启用监控列表自动刷新', 'info');
  } else {
    log('已禁用监控列表自动刷新', 'info');
  }
}

// 过滤结果
function filterResults() {
  const filter = document.getElementById('filterInput').value.toLowerCase();
  const rows = document.querySelectorAll('#resultsBody tr');

  rows.forEach(row => {
    const address = row.querySelector('.address-cell')?.textContent.toLowerCase() || '';
    if (address.includes(filter)) {
      row.style.display = '';
    } else {
      row.style.display = 'none';
    }
  });
}

// 导出结果
async function exportResults() {
  try {
    const json = JSON.stringify(currentResults, null, 2);
    const blob = new Blob([json], { type: 'application/json' });
    const url = URL.createObjectURL(blob);

    const a = document.createElement('a');
    a.href = url;
    a.download = `wasm-scan-results-${Date.now()}.json`;
    a.click();

    URL.revokeObjectURL(url);
    log(`已导出 ${currentResults.length} 个结果`, 'success');
  } catch (e) {
    log(`导出失败: ${e.message}`, 'error');
  }
}

// 日志
function log(message, level = 'info') {
  const container = document.getElementById('logContainer');
  const entry = document.createElement('div');
  entry.className = `log-entry ${level}`;

  const time = new Date().toLocaleTimeString();
  entry.innerHTML = `<span class="log-time">[${time}]</span>${message}`;

  container.appendChild(entry);
  container.scrollTop = container.scrollHeight;

  console.log(`[WASM Scanner] ${message}`);
}

// 清空日志
function clearLog() {
  document.getElementById('logContainer').innerHTML = '';
}

// 定期检查连接状态
setInterval(checkConnection, 5000);
