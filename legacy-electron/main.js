// main.js
// Processo principal do Electron: cria a janela do app e responde às
// chamadas do processo renderer (interface) via IPC, lendo/gravando no
// banco de dados local (db.js).

const { app, BrowserWindow, ipcMain } = require('electron');
const path = require('path');
const { createDb, nextId } = require('./db');

let mainWindow;
let db;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 1024,
    minHeight: 640,
    backgroundColor: '#f4f6f9',
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  });

  mainWindow.setMenuBarVisibility(false);
  mainWindow.loadFile(path.join(__dirname, 'renderer', 'index.html'));
}

app.whenReady().then(() => {
  db = createDb(app.getPath('userData'));
  registerIpcHandlers();
  createWindow();

  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit();
});

// ---------------------------------------------------------------------
// Helpers de domínio
// ---------------------------------------------------------------------

function todayISO() {
  return new Date().toISOString().slice(0, 10);
}

// Deriva o status de um lançamento a partir das datas, em vez de guardar um
// campo "status" que poderia ficar desatualizado.
function statusDoLancamento(lanc) {
  if (lanc.dataPagamento) return lanc.tipo === 'pagar' ? 'pago' : 'recebido';
  if (lanc.dataVencimento < todayISO()) return 'atrasado';
  return 'pendente';
}

function comStatus(lanc) {
  return { ...lanc, status: statusDoLancamento(lanc) };
}

function mesAno(dataISO) {
  return dataISO.slice(0, 7); // "YYYY-MM"
}

// ---------------------------------------------------------------------
// IPC handlers
// ---------------------------------------------------------------------

function registerIpcHandlers() {
  // --- Contas bancárias / caixas ---
  ipcMain.handle('contas:list', () => db.get('contas').value());

  ipcMain.handle('contas:create', (e, payload) => {
    const id = nextId(db, 'contas');
    const conta = { id, nome: payload.nome, saldoInicial: Number(payload.saldoInicial) || 0 };
    db.get('contas').push(conta).write();
    return conta;
  });

  ipcMain.handle('contas:update', (e, payload) => {
    db.get('contas').find({ id: payload.id }).assign(payload).write();
    return db.get('contas').find({ id: payload.id }).value();
  });

  ipcMain.handle('contas:delete', (e, id) => {
    db.get('contas').remove({ id }).write();
    return true;
  });

  // --- Categorias (plano de contas simplificado) ---
  ipcMain.handle('categorias:list', () => db.get('categorias').value());

  ipcMain.handle('categorias:create', (e, payload) => {
    const id = nextId(db, 'categorias');
    const categoria = { id, nome: payload.nome, tipo: payload.tipo };
    db.get('categorias').push(categoria).write();
    return categoria;
  });

  ipcMain.handle('categorias:delete', (e, id) => {
    db.get('categorias').remove({ id }).write();
    return true;
  });

  // --- Lançamentos (contas a pagar / a receber) ---
  ipcMain.handle('lancamentos:list', (e, filtros = {}) => {
    let itens = db.get('lancamentos').value().map(comStatus);

    if (filtros.tipo) itens = itens.filter(l => l.tipo === filtros.tipo);
    if (filtros.status) itens = itens.filter(l => l.status === filtros.status);
    if (filtros.dataInicio) itens = itens.filter(l => l.dataVencimento >= filtros.dataInicio);
    if (filtros.dataFim) itens = itens.filter(l => l.dataVencimento <= filtros.dataFim);

    itens.sort((a, b) => a.dataVencimento.localeCompare(b.dataVencimento));
    return itens;
  });

  ipcMain.handle('lancamentos:create', (e, payload) => {
    const id = nextId(db, 'lancamentos');
    const lanc = {
      id,
      tipo: payload.tipo, // 'pagar' | 'receber'
      descricao: payload.descricao,
      categoriaId: payload.categoriaId || null,
      contaId: payload.contaId || null,
      valor: Number(payload.valor) || 0,
      dataVencimento: payload.dataVencimento,
      dataPagamento: null,
      observacoes: payload.observacoes || ''
    };
    db.get('lancamentos').push(lanc).write();
    return comStatus(lanc);
  });

  ipcMain.handle('lancamentos:update', (e, payload) => {
    db.get('lancamentos').find({ id: payload.id }).assign(payload).write();
    return comStatus(db.get('lancamentos').find({ id: payload.id }).value());
  });

  ipcMain.handle('lancamentos:delete', (e, id) => {
    db.get('lancamentos').remove({ id }).write();
    return true;
  });

  ipcMain.handle('lancamentos:marcarBaixa', (e, { id, dataPagamento }) => {
    const data = dataPagamento || todayISO();
    db.get('lancamentos').find({ id }).assign({ dataPagamento: data }).write();
    return comStatus(db.get('lancamentos').find({ id }).value());
  });

  ipcMain.handle('lancamentos:estornar', (e, id) => {
    db.get('lancamentos').find({ id }).assign({ dataPagamento: null }).write();
    return comStatus(db.get('lancamentos').find({ id }).value());
  });

  // --- Dashboard ---
  ipcMain.handle('dashboard:resumo', () => {
    const contas = db.get('contas').value();
    const lancamentos = db.get('lancamentos').value().map(comStatus);

    const saldoInicial = contas.reduce((s, c) => s + (c.saldoInicial || 0), 0);
    const totalPago = lancamentos
      .filter(l => l.tipo === 'pagar' && l.dataPagamento)
      .reduce((s, l) => s + l.valor, 0);
    const totalRecebido = lancamentos
      .filter(l => l.tipo === 'receber' && l.dataPagamento)
      .reduce((s, l) => s + l.valor, 0);
    const saldoAtual = saldoInicial + totalRecebido - totalPago;

    const totalAPagar = lancamentos
      .filter(l => l.tipo === 'pagar' && l.status !== 'pago')
      .reduce((s, l) => s + l.valor, 0);
    const totalAReceber = lancamentos
      .filter(l => l.tipo === 'receber' && l.status !== 'recebido')
      .reduce((s, l) => s + l.valor, 0);

    const hoje = todayISO();
    const proximosVencimentos = lancamentos
      .filter(l => l.status !== 'pago' && l.status !== 'recebido')
      .sort((a, b) => a.dataVencimento.localeCompare(b.dataVencimento))
      .slice(0, 8);

    // Fluxo dos últimos 6 meses (entradas x saídas, por competência = vencimento)
    const meses = ultimosMeses(6);
    const fluxoMensal = meses.map(m => {
      const doMes = lancamentos.filter(l => mesAno(l.dataVencimento) === m);
      const entradas = doMes.filter(l => l.tipo === 'receber').reduce((s, l) => s + l.valor, 0);
      const saidas = doMes.filter(l => l.tipo === 'pagar').reduce((s, l) => s + l.valor, 0);
      return { mes: m, entradas, saidas };
    });

    return {
      saldoAtual,
      totalAPagar,
      totalAReceber,
      proximosVencimentos,
      fluxoMensal,
      hoje
    };
  });

  // --- Relatório: Fluxo de Caixa mensal detalhado (ano informado) ---
  ipcMain.handle('relatorios:fluxoCaixa', (e, { ano }) => {
    const lancamentos = db.get('lancamentos').value().map(comStatus);
    const contas = db.get('contas').value();
    const saldoInicial = contas.reduce((s, c) => s + (c.saldoInicial || 0), 0);

    const meses = Array.from({ length: 12 }, (_, i) => `${ano}-${String(i + 1).padStart(2, '0')}`);
    let saldoAcumulado = saldoInicial;

    return meses.map(m => {
      const doMes = lancamentos.filter(l => mesAno(l.dataVencimento) === m);
      const entradas = doMes.filter(l => l.tipo === 'receber').reduce((s, l) => s + l.valor, 0);
      const saidas = doMes.filter(l => l.tipo === 'pagar').reduce((s, l) => s + l.valor, 0);
      const saldoPeriodo = entradas - saidas;
      saldoAcumulado += saldoPeriodo;
      return { mes: m, entradas, saidas, saldoPeriodo, saldoAcumulado };
    });
  });

  // --- Relatório: DRE simplificado por período ---
  ipcMain.handle('relatorios:dre', (e, { dataInicio, dataFim }) => {
    const categorias = db.get('categorias').value();
    const lancamentos = db.get('lancamentos').value().map(comStatus)
      .filter(l => l.dataVencimento >= dataInicio && l.dataVencimento <= dataFim);

    const linhasPorCategoria = categorias.map(cat => {
      const total = lancamentos
        .filter(l => l.categoriaId === cat.id)
        .reduce((s, l) => s + l.valor, 0);
      return { categoria: cat.nome, tipo: cat.tipo, total };
    }).filter(l => l.total > 0);

    const receitaBruta = linhasPorCategoria
      .filter(l => l.tipo === 'receita')
      .reduce((s, l) => s + l.total, 0);
    const despesas = linhasPorCategoria
      .filter(l => l.tipo === 'despesa')
      .reduce((s, l) => s + l.total, 0);
    const resultado = receitaBruta - despesas;

    return { linhas: linhasPorCategoria, receitaBruta, despesas, resultado };
  });
}

function ultimosMeses(qtd) {
  const out = [];
  const d = new Date();
  for (let i = qtd - 1; i >= 0; i--) {
    const dt = new Date(d.getFullYear(), d.getMonth() - i, 1);
    out.push(`${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, '0')}`);
  }
  return out;
}
