// preload.js
// All Node.js APIs are available in the preload process.
// It has the same sandbox as a Chrome extension.

window.addEventListener('DOMContentLoaded', () => {
  console.log('Empower1 Wallet preload script has been loaded.');

  // Example of exposing a simple API to the renderer process via contextBridge
  // (Not strictly needed for this subtask's core logic if renderer directly uses 'elliptic',
  // but good practice for future secure IPC with main process for file ops, etc.)
  /*
  const { contextBridge, ipcRenderer } = require('electron');

  contextBridge.exposeInMainWorld('electronAPI', {
    // Example function:
    // saveKeyToFile: (keyData, password) => ipcRenderer.invoke('save-key', keyData, password),
    // loadKeyFromFile: () => ipcRenderer.invoke('load-key'),
    // You would then handle these 'save-key', 'load-key' channels in main.js
    // using ipcMain.handle(...)
  });
  */
});
