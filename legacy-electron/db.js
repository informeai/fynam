// db.js
// Camada de dados local do XFin Desktop.
// Usa lowdb (v1, síncrono) para persistir tudo em um arquivo JSON dentro da
// pasta de dados do usuário do Electron (app.getPath('userData')).
//
// Este arquivo roda no processo "main" do Electron. A interface exposta ao
// processo "renderer" (a interface visual) fica em preload.js + main.js,
// que chamam as funções daqui via IPC.

const path = require('path');
const low = require('lowdb');
const FileSync = require('lowdb/adapters/FileSync');

function createDb(userDataPath) {
  const file = path.join(userDataPath, 'xfin-data.json');
  const adapter = new FileSync(file);
  const db = low(adapter);

  db.defaults({
    contas: [],
    categorias: [],
    lancamentos: [],
    nextIds: { contas: 1, categorias: 1, lancamentos: 1 }
  }).write();

  seedIfEmpty(db);

  return db;
}

// Na primeira execução, cria uma conta padrão e um plano de contas básico
// para o usuário não começar com as telas totalmente vazias.
function seedIfEmpty(db) {
  if (db.get('contas').size().value() === 0) {
    const contaId = nextId(db, 'contas');
    db.get('contas')
      .push({ id: contaId, nome: 'Caixa / Conta Principal', saldoInicial: 0 })
      .write();
  }

  if (db.get('categorias').size().value() === 0) {
    const categoriasIniciais = [
      { nome: 'Vendas', tipo: 'receita' },
      { nome: 'Serviços prestados', tipo: 'receita' },
      { nome: 'Outras receitas', tipo: 'receita' },
      { nome: 'Fornecedores', tipo: 'despesa' },
      { nome: 'Folha de pagamento', tipo: 'despesa' },
      { nome: 'Aluguel', tipo: 'despesa' },
      { nome: 'Impostos', tipo: 'despesa' },
      { nome: 'Marketing', tipo: 'despesa' },
      { nome: 'Outras despesas', tipo: 'despesa' }
    ];
    categoriasIniciais.forEach(cat => {
      const id = nextId(db, 'categorias');
      db.get('categorias').push({ id, ...cat }).write();
    });
  }
}

function nextId(db, collection) {
  const id = db.get(`nextIds.${collection}`).value();
  db.set(`nextIds.${collection}`, id + 1).write();
  return id;
}

module.exports = { createDb, nextId };
