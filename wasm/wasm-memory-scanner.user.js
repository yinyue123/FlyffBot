// ==UserScript==
// @name         WASM Memory Scanner
// @namespace    http://tampermonkey.net/
// @version      1.0
// @description  Search and modify WebAssembly memory like Cheat Engine
// @author       You
// @match        *://*/*
// @grant        none
// ==/UserScript==

(function() {
    'use strict';

    class WASMMemoryScanner {
        constructor() {
            this.wasmInstances = [];
            this.searchResults = [];
            this.lastSearchResults = [];
            this.scanType = 'exact'; // exact, fuzzy, changed, unchanged
            this.valueType = 'int32'; // int8, int16, int32, float32, float64
            this.scanInterval = null;
            this.initUI();
            this.hookWASM();
            this.startContinuousScanning();
        }

        hookWASM() {
            const self = this;
            const originalInstantiate = WebAssembly.instantiate;
            const originalInstantiateStreaming = WebAssembly.instantiateStreaming;

            WebAssembly.instantiate = async function(...args) {
                console.log('[WASM Scanner] Intercepting WebAssembly.instantiate');
                const result = await originalInstantiate.apply(this, args);
                if (result.instance) {
                    self.registerInstance(result.instance, 'instantiate');
                }
                return result;
            };

            WebAssembly.instantiateStreaming = async function(...args) {
                console.log('[WASM Scanner] Intercepting WebAssembly.instantiateStreaming');
                const result = await originalInstantiateStreaming.apply(this, args);
                if (result.instance) {
                    self.registerInstance(result.instance, 'instantiateStreaming');
                }
                return result;
            };

            console.log('[WASM Scanner] WebAssembly hooks installed');
        }

        registerInstance(instance, source) {
            const memory = instance.exports.memory || this.findMemoryInExports(instance.exports);
            if (memory && memory.buffer) {
                // 检查是否已存在
                const exists = this.wasmInstances.some(inst => inst.memory === memory);
                if (exists) {
                    console.log('[WASM Scanner] Memory already registered');
                    return;
                }

                this.wasmInstances.push({
                    instance: instance,
                    memory: memory,
                    name: `Instance ${this.wasmInstances.length + 1}`,
                    source: source || 'unknown'
                });
                this.updateInstanceList();
                console.log(`[WASM Scanner] Registered from ${source}:`, (memory.buffer.byteLength / 1024 / 1024).toFixed(2), 'MB');
            }
        }

        registerMemoryBuffer(buffer, source) {
            // 检查是否已存在
            const exists = this.wasmInstances.some(inst => inst.memory.buffer === buffer);
            if (exists) {
                console.log('[WASM Scanner] Buffer already registered');
                return;
            }

            // 创建伪Memory对象
            const fakeMemory = {
                buffer: buffer,
                grow: function() { throw new Error('Cannot grow fake memory'); }
            };

            this.wasmInstances.push({
                instance: null,
                memory: fakeMemory,
                name: `Buffer ${this.wasmInstances.length + 1}`,
                source: source,
                isFake: true
            });
            this.updateInstanceList();
            console.log(`[WASM Scanner] Registered buffer from ${source}:`, (buffer.byteLength / 1024 / 1024).toFixed(2), 'MB');
        }

        findMemoryInExports(exports) {
            for (let key in exports) {
                if (exports[key] instanceof WebAssembly.Memory) {
                    return exports[key];
                }
            }
            return null;
        }

        // 扫描现有的WASM实例
        scanExistingWasm() {
            console.log('[WASM Scanner] Scanning for existing WASM instances...');
            let foundCount = 0;

            // 方法1: Unity/Emscripten常见位置
            const unityLocations = ['unityInstance', 'gameInstance', 'Module'];
            for (const key of unityLocations) {
                try {
                    if (window[key]) {
                        console.log(`[WASM Scanner] Found ${key}`);

                        // 检查Module.HEAPU8.buffer
                        if (window[key].Module && window[key].Module.HEAPU8) {
                            const buffer = window[key].Module.HEAPU8.buffer;
                            if (buffer instanceof ArrayBuffer || buffer instanceof SharedArrayBuffer) {
                                this.registerMemoryBuffer(buffer, `${key}.Module.HEAPU8`);
                                foundCount++;
                            }
                        }

                        // 检查memory属性
                        if (window[key].memory instanceof WebAssembly.Memory) {
                            this.registerMemoryBuffer(window[key].memory.buffer, `${key}.memory`);
                            foundCount++;
                        }
                    }
                } catch (e) {
                    console.log(`[WASM Scanner] Error checking ${key}:`, e.message);
                }
            }

            // 方法2: 全局Module (Emscripten)
            if (typeof Module !== 'undefined') {
                console.log('[WASM Scanner] Found global Module');
                try {
                    if (Module.HEAPU8 && Module.HEAPU8.buffer) {
                        this.registerMemoryBuffer(Module.HEAPU8.buffer, 'Module.HEAPU8');
                        foundCount++;
                    }
                    if (Module.asm && Module.asm.memory instanceof WebAssembly.Memory) {
                        this.registerMemoryBuffer(Module.asm.memory.buffer, 'Module.asm.memory');
                        foundCount++;
                    }
                    if (Module.wasmMemory instanceof WebAssembly.Memory) {
                        this.registerMemoryBuffer(Module.wasmMemory.buffer, 'Module.wasmMemory');
                        foundCount++;
                    }
                } catch (e) {
                    console.log('[WASM Scanner] Error checking Module:', e.message);
                }
            }

            // 方法3: 其他常见位置
            const otherLocations = ['wasmMemory', 'WasmInstance', '_wasmInstance', 'instance', 'exports'];
            for (const key of otherLocations) {
                try {
                    if (window[key] instanceof WebAssembly.Memory) {
                        this.registerMemoryBuffer(window[key].buffer, `global.${key}`);
                        foundCount++;
                    }
                } catch (e) {
                    // 忽略
                }
            }

            console.log(`[WASM Scanner] Scan complete. Found ${foundCount} new instances. Total: ${this.wasmInstances.length}`);
            return foundCount;
        }

        getSelectedMemory() {
            const select = document.getElementById('wasm-instance-select');
            const index = parseInt(select.value);
            return this.wasmInstances[index]?.memory;
        }

        searchMemory(value) {
            const memory = this.getSelectedMemory();
            if (!memory) {
                alert('No WASM instance selected!');
                return;
            }

            this.searchResults = [];
            const buffer = new Uint8Array(memory.buffer);
            const dataView = new DataView(memory.buffer);

            const readValue = (offset) => {
                switch(this.valueType) {
                    case 'int8': return dataView.getInt8(offset);
                    case 'uint8': return dataView.getUint8(offset);
                    case 'int16': return dataView.getInt16(offset, true);
                    case 'uint16': return dataView.getUint16(offset, true);
                    case 'int32': return dataView.getInt32(offset, true);
                    case 'uint32': return dataView.getUint32(offset, true);
                    case 'float32': return dataView.getFloat32(offset, true);
                    case 'float64': return dataView.getFloat64(offset, true);
                    default: return dataView.getInt32(offset, true);
                }
            };

            const getValueSize = () => {
                switch(this.valueType) {
                    case 'int8':
                    case 'uint8': return 1;
                    case 'int16':
                    case 'uint16': return 2;
                    case 'int32':
                    case 'uint32':
                    case 'float32': return 4;
                    case 'float64': return 8;
                    default: return 4;
                }
            };

            const valueSize = getValueSize();
            const searchValue = this.valueType.includes('float') ? parseFloat(value) : parseInt(value);

            if (this.scanType === 'exact') {
                for (let i = 0; i < buffer.length - valueSize; i += valueSize) {
                    try {
                        const memValue = readValue(i);
                        if (memValue === searchValue) {
                            this.searchResults.push({
                                address: i,
                                value: memValue
                            });
                        }
                    } catch(e) {}
                }
            } else if (this.scanType === 'fuzzy') {
                const tolerance = parseFloat(document.getElementById('fuzzy-tolerance').value) || 0;
                for (let i = 0; i < buffer.length - valueSize; i += valueSize) {
                    try {
                        const memValue = readValue(i);
                        if (Math.abs(memValue - searchValue) <= tolerance) {
                            this.searchResults.push({
                                address: i,
                                value: memValue
                            });
                        }
                    } catch(e) {}
                }
            } else if (this.scanType === 'changed' || this.scanType === 'unchanged') {
                if (this.lastSearchResults.length === 0) {
                    alert('Please run an initial scan first!');
                    return;
                }

                for (let result of this.lastSearchResults) {
                    try {
                        const currentValue = readValue(result.address);
                        const hasChanged = currentValue !== result.value;

                        if ((this.scanType === 'changed' && hasChanged) ||
                            (this.scanType === 'unchanged' && !hasChanged)) {
                            this.searchResults.push({
                                address: result.address,
                                value: currentValue
                            });
                        }
                    } catch(e) {}
                }
            }

            this.lastSearchResults = [...this.searchResults];
            this.displayResults();
        }

        displayResults() {
            const resultsDiv = document.getElementById('scan-results');
            const resultCount = document.getElementById('result-count');

            resultCount.textContent = `Found ${this.searchResults.length} results`;

            if (this.searchResults.length > 1000) {
                resultsDiv.innerHTML = `<div style="color: #ff9800; padding: 10px;">Too many results (${this.searchResults.length}). Please refine your search.</div>`;
                return;
            }

            resultsDiv.innerHTML = '';
            const fragment = document.createDocumentFragment();

            this.searchResults.slice(0, 1000).forEach(result => {
                const resultItem = document.createElement('div');
                resultItem.className = 'result-item';
                resultItem.innerHTML = `
                    <span class="address">0x${result.address.toString(16).padStart(8, '0')}</span>
                    <span class="value">${result.value}</span>
                    <button class="modify-btn" data-address="${result.address}">Modify</button>
                    <button class="watch-btn" data-address="${result.address}">Watch</button>
                `;
                fragment.appendChild(resultItem);
            });

            resultsDiv.appendChild(fragment);
        }

        modifyMemory(address, newValue) {
            const memory = this.getSelectedMemory();
            if (!memory) return;

            const dataView = new DataView(memory.buffer);
            const value = this.valueType.includes('float') ? parseFloat(newValue) : parseInt(newValue);

            try {
                switch(this.valueType) {
                    case 'int8': dataView.setInt8(address, value); break;
                    case 'uint8': dataView.setUint8(address, value); break;
                    case 'int16': dataView.setInt16(address, value, true); break;
                    case 'uint16': dataView.setUint16(address, value, true); break;
                    case 'int32': dataView.setInt32(address, value, true); break;
                    case 'uint32': dataView.setUint32(address, value, true); break;
                    case 'float32': dataView.setFloat32(address, value, true); break;
                    case 'float64': dataView.setFloat64(address, value, true); break;
                }
                alert(`Memory at 0x${address.toString(16)} modified to ${value}`);
                this.searchResults = this.searchResults.filter(r => r.address !== address);
                this.displayResults();
            } catch(e) {
                alert('Failed to modify memory: ' + e.message);
            }
        }

        startContinuousScanning() {
            // 立即扫描一次
            this.scanExistingWasm();

            // 每2秒扫描一次
            this.scanInterval = setInterval(() => {
                this.scanExistingWasm();
            }, 2000);

            // 也在特定时间点扫描
            setTimeout(() => this.scanExistingWasm(), 5000);
            setTimeout(() => this.scanExistingWasm(), 10000);
            setTimeout(() => this.scanExistingWasm(), 15000);
        }

        updateInstanceList() {
            const select = document.getElementById('wasm-instance-select');
            const statusDiv = document.getElementById('wasm-status');

            if (this.wasmInstances.length === 0) {
                select.innerHTML = '<option>Scanning for WASM instances...</option>';
                if (statusDiv) {
                    statusDiv.textContent = 'Status: Scanning...';
                    statusDiv.style.color = '#FFA500';
                }
            } else {
                select.innerHTML = '';
                this.wasmInstances.forEach((inst, index) => {
                    const option = document.createElement('option');
                    option.value = index;
                    const sizeMB = (inst.memory.buffer.byteLength / 1024 / 1024).toFixed(2);
                    option.textContent = `${inst.source} (${sizeMB} MB)`;
                    select.appendChild(option);
                });
                if (statusDiv) {
                    statusDiv.textContent = `Status: Found ${this.wasmInstances.length} instance(s)`;
                    statusDiv.style.color = '#4CAF50';
                }
            }
        }

        initUI() {
            const panel = document.createElement('div');
            panel.id = 'wasm-scanner-panel';
            panel.innerHTML = `
                <style>
                    #wasm-scanner-panel {
                        position: fixed;
                        top: 10px;
                        right: 10px;
                        width: 400px;
                        background: #1e1e1e;
                        color: #fff;
                        border: 2px solid #333;
                        border-radius: 8px;
                        padding: 15px;
                        font-family: 'Consolas', 'Monaco', monospace;
                        font-size: 12px;
                        z-index: 999999;
                        max-height: 80vh;
                        overflow-y: auto;
                        box-shadow: 0 4px 20px rgba(0,0,0,0.5);
                    }
                    #wasm-scanner-panel.minimized {
                        width: 200px;
                        height: auto;
                    }
                    #wasm-scanner-panel.minimized .panel-content {
                        display: none;
                    }
                    .panel-header {
                        display: flex;
                        justify-content: space-between;
                        align-items: center;
                        margin-bottom: 15px;
                        padding-bottom: 10px;
                        border-bottom: 1px solid #444;
                    }
                    .panel-title {
                        font-size: 14px;
                        font-weight: bold;
                        color: #4CAF50;
                    }
                    .panel-controls button {
                        background: #333;
                        border: 1px solid #555;
                        color: #fff;
                        padding: 2px 8px;
                        margin-left: 5px;
                        cursor: pointer;
                        border-radius: 3px;
                    }
                    .panel-controls button:hover {
                        background: #444;
                    }
                    .control-group {
                        margin-bottom: 12px;
                    }
                    .control-group label {
                        display: block;
                        margin-bottom: 5px;
                        color: #bbb;
                        font-size: 11px;
                    }
                    .control-group select,
                    .control-group input {
                        width: 100%;
                        padding: 6px;
                        background: #2a2a2a;
                        border: 1px solid #444;
                        color: #fff;
                        border-radius: 4px;
                        box-sizing: border-box;
                    }
                    .control-row {
                        display: flex;
                        gap: 8px;
                    }
                    .control-row > * {
                        flex: 1;
                    }
                    button {
                        padding: 8px 12px;
                        background: #4CAF50;
                        color: white;
                        border: none;
                        border-radius: 4px;
                        cursor: pointer;
                        font-size: 11px;
                        font-weight: bold;
                        transition: background 0.2s;
                    }
                    button:hover {
                        background: #45a049;
                    }
                    button:active {
                        background: #3d8b40;
                    }
                    #result-count {
                        color: #4CAF50;
                        margin: 10px 0;
                        font-weight: bold;
                    }
                    #scan-results {
                        max-height: 300px;
                        overflow-y: auto;
                        background: #252525;
                        border: 1px solid #333;
                        border-radius: 4px;
                        padding: 5px;
                    }
                    .result-item {
                        padding: 6px;
                        margin: 3px 0;
                        background: #2a2a2a;
                        border-radius: 3px;
                        display: flex;
                        align-items: center;
                        gap: 8px;
                    }
                    .result-item:hover {
                        background: #333;
                    }
                    .address {
                        color: #64B5F6;
                        font-family: monospace;
                        min-width: 90px;
                    }
                    .value {
                        color: #FFD54F;
                        flex: 1;
                    }
                    .modify-btn, .watch-btn {
                        padding: 3px 8px;
                        background: #FF9800;
                        font-size: 10px;
                    }
                    .watch-btn {
                        background: #2196F3;
                    }
                    .modify-btn:hover {
                        background: #FB8C00;
                    }
                    .watch-btn:hover {
                        background: #1976D2;
                    }
                </style>

                <div class="panel-header">
                    <div class="panel-title">WASM Memory Scanner</div>
                    <div class="panel-controls">
                        <button id="minimize-btn">_</button>
                        <button id="close-btn">X</button>
                    </div>
                </div>

                <div class="panel-content">
                    <div id="wasm-status" style="color: #FFA500; margin-bottom: 10px; font-size: 11px;">
                        Status: Initializing...
                    </div>

                    <div class="control-group">
                        <label>WASM Instance:</label>
                        <div style="display: flex; gap: 5px;">
                            <select id="wasm-instance-select" style="flex: 1;">
                                <option>Scanning...</option>
                            </select>
                            <button id="refresh-btn" style="padding: 6px 10px; flex: 0;">🔄</button>
                        </div>
                    </div>

                    <div class="control-row">
                        <div class="control-group">
                            <label>Value Type:</label>
                            <select id="value-type">
                                <option value="int8">Int8</option>
                                <option value="uint8">UInt8</option>
                                <option value="int16">Int16</option>
                                <option value="uint16">UInt16</option>
                                <option value="int32" selected>Int32</option>
                                <option value="uint32">UInt32</option>
                                <option value="float32">Float32</option>
                                <option value="float64">Float64</option>
                            </select>
                        </div>

                        <div class="control-group">
                            <label>Scan Type:</label>
                            <select id="scan-type">
                                <option value="exact">Exact Value</option>
                                <option value="fuzzy">Fuzzy Search</option>
                                <option value="changed">Changed Values</option>
                                <option value="unchanged">Unchanged Values</option>
                            </select>
                        </div>
                    </div>

                    <div class="control-group" id="fuzzy-group" style="display: none;">
                        <label>Tolerance:</label>
                        <input type="number" id="fuzzy-tolerance" value="0" step="0.1">
                    </div>

                    <div class="control-group">
                        <label>Search Value:</label>
                        <input type="text" id="search-value" placeholder="Enter value to search">
                    </div>

                    <div class="control-row">
                        <button id="first-scan-btn">First Scan</button>
                        <button id="next-scan-btn">Next Scan</button>
                        <button id="reset-btn">Reset</button>
                    </div>

                    <div id="result-count">Ready to scan</div>
                    <div id="scan-results"></div>
                </div>
            `;

            document.body.appendChild(panel);

            // Event listeners
            document.getElementById('close-btn').addEventListener('click', () => {
                panel.classList.toggle('minimized');
            });

            document.getElementById('minimize-btn').addEventListener('click', () => {
                panel.classList.toggle('minimized');
            });

            document.getElementById('refresh-btn').addEventListener('click', () => {
                console.log('[WASM Scanner] Manual refresh triggered');
                this.scanExistingWasm();
            });

            document.getElementById('value-type').addEventListener('change', (e) => {
                this.valueType = e.target.value;
            });

            document.getElementById('scan-type').addEventListener('change', (e) => {
                this.scanType = e.target.value;
                const fuzzyGroup = document.getElementById('fuzzy-group');
                fuzzyGroup.style.display = this.scanType === 'fuzzy' ? 'block' : 'none';
            });

            document.getElementById('first-scan-btn').addEventListener('click', () => {
                const value = document.getElementById('search-value').value;
                if ((this.scanType === 'exact' || this.scanType === 'fuzzy') && !value) {
                    alert('Please enter a search value!');
                    return;
                }
                this.lastSearchResults = [];
                if (this.scanType === 'exact' || this.scanType === 'fuzzy') {
                    this.searchMemory(value);
                } else {
                    // For changed/unchanged, we need to store initial values
                    this.scanType = 'exact';
                    this.searchMemory(value);
                    this.scanType = document.getElementById('scan-type').value;
                }
            });

            document.getElementById('next-scan-btn').addEventListener('click', () => {
                const value = document.getElementById('search-value').value;
                if (this.lastSearchResults.length === 0) {
                    alert('Please run First Scan first!');
                    return;
                }
                this.searchMemory(value);
            });

            document.getElementById('reset-btn').addEventListener('click', () => {
                this.searchResults = [];
                this.lastSearchResults = [];
                this.displayResults();
                document.getElementById('result-count').textContent = 'Ready to scan';
                document.getElementById('search-value').value = '';
            });

            // Delegate event for modify buttons
            document.getElementById('scan-results').addEventListener('click', (e) => {
                if (e.target.classList.contains('modify-btn')) {
                    const address = parseInt(e.target.dataset.address);
                    const newValue = prompt('Enter new value:');
                    if (newValue !== null) {
                        this.modifyMemory(address, newValue);
                    }
                } else if (e.target.classList.contains('watch-btn')) {
                    const address = parseInt(e.target.dataset.address);
                    alert('Watch feature coming soon!\nAddress: 0x' + address.toString(16));
                }
            });

            // Make panel draggable
            this.makeDraggable(panel);
        }

        makeDraggable(element) {
            let pos1 = 0, pos2 = 0, pos3 = 0, pos4 = 0;
            const header = element.querySelector('.panel-header');

            header.onmousedown = dragMouseDown;

            function dragMouseDown(e) {
                e.preventDefault();
                pos3 = e.clientX;
                pos4 = e.clientY;
                document.onmouseup = closeDragElement;
                document.onmousemove = elementDrag;
            }

            function elementDrag(e) {
                e.preventDefault();
                pos1 = pos3 - e.clientX;
                pos2 = pos4 - e.clientY;
                pos3 = e.clientX;
                pos4 = e.clientY;
                element.style.top = (element.offsetTop - pos2) + "px";
                element.style.left = (element.offsetLeft - pos1) + "px";
                element.style.right = "auto";
            }

            function closeDragElement() {
                document.onmouseup = null;
                document.onmousemove = null;
            }
        }
    }

    // Initialize scanner - try multiple approaches
    function initScanner() {
        if (window.wasmScanner) {
            console.log('WASM Memory Scanner already initialized!');
            return;
        }

        try {
            window.wasmScanner = new WASMMemoryScanner();
            console.log('✓ WASM Memory Scanner initialized!');
            console.log('Press Ctrl+Shift+M to toggle the panel');
        } catch(e) {
            console.error('Failed to initialize WASM Memory Scanner:', e);
        }
    }

    // Try to initialize immediately
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initScanner);
    } else {
        // DOM is already loaded
        initScanner();
    }

    // Backup: also try on window load
    window.addEventListener('load', () => {
        setTimeout(initScanner, 100);
    });
})();
