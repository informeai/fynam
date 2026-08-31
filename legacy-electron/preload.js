// preload.js
// Expõe de forma segura (contextIsolation) apenas as funções necessárias
// para a interface (renderer) conversar com o processo main via IPC.
// Nada de Node/fs é exposto diretamente à página.

const { contextBridge, ipcRenderer } = require('electron');

function invoke(channel) {
  return (...args) => ipcRenderer.invoke(channel, ...args);
}

contextBridge.exposeInMainWorld('xfin', {
  contas: {
    list: invoke('contas:list'),
    create: invoke('contas:create'),
    update: invoke('contas:update'),
    delete: invoke('contas:delete')
  },
  categorias: {
    list: invoke('categorias:list'),
    create: invoke('categorias:create'),
    delete: invoke('categorias:delete')
  },
  lancamentos: {
    list: invoke('lancamentos:list'),
    create: invoke('lancamentos:create'),
    update: invoke('lancamentos:update'),
    delete: invoke('lancamentos:delete'),
    marcarBaixa: invoke('lancamentos:marcarBaixa'),
    estornar: invoke('lancamentos:estornar')
  },
  dashboard: {
    resumo: invoke('dashboard:resumo')
  },
  relatorios: {
    fluxoCaixa: invoke('relatorios:fluxoCaixa'),
    dre: invoke('relatorios:dre')
  }
});
