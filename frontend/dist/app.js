// app.js
// Lógica da interface. Fala com o backend Go somente através dos métodos
// que o Wails expõe em window.go.main.App.*

(() => {
  const App = window.go.main.App;

  const state = {
    categorias: [],
    contas: [],
    empresas: [],
    empresaAtivaId: null,
    filtroPagar: 'todos',
    filtroReceber: 'todos'
  };

  const currency = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' });
  const fmtMoney = (v) => currency.format(v || 0);
  const fmtDate = (iso) => {
    if (!iso) return '—';
    const [y, m, d] = iso.split('-');
    return `${d}/${m}/${y}`;
  };
  const monthLabel = (mesAno) => {
    const [y, m] = mesAno.split('-');
    const nomes = ['Jan', 'Fev', 'Mar', 'Abr', 'Mai', 'Jun', 'Jul', 'Ago', 'Set', 'Out', 'Nov', 'Dez'];
    return `${nomes[Number(m) - 1]}/${y.slice(2)}`;
  };

  // ---------------------------------------------------------------
  // Navegação entre páginas
  // ---------------------------------------------------------------

  function setupNav() {
    document.querySelectorAll('.nav-item').forEach((btn) => {
      btn.addEventListener('click', () => goToPage(btn.dataset.page));
    });
  }

  function goToPage(page) {
    document.querySelectorAll('.nav-item').forEach((b) => b.classList.toggle('active', b.dataset.page === page));
    document.querySelectorAll('.page').forEach((p) => p.classList.toggle('active', p.id === `page-${page}`));

    if (page === 'dashboard') loadDashboard();
    if (page === 'pagar') loadLancamentos('pagar');
    if (page === 'receber') loadLancamentos('receber');
    if (page === 'fluxo') loadFluxo();
    if (page === 'dre') loadDre();
    if (page === 'empresas') carregarEmpresas();
    if (page === 'cadastros') loadCadastros();
  }

  function paginaAtual() {
    const ativa = document.querySelector('.nav-item.active');
    return ativa ? ativa.dataset.page : 'dashboard';
  }

  // ---------------------------------------------------------------
  // Empresas (múltiplas empresas / filiais)
  // ---------------------------------------------------------------

  function setupEmpresas() {
    document.getElementById('empresa-atual').addEventListener('change', async (e) => {
      try {
        await App.TrocarEmpresa(Number(e.target.value));
      } catch (err) {
        toast('Não foi possível trocar de empresa: ' + err, true);
      }
    });

    document.getElementById('form-empresa').addEventListener('submit', async (e) => {
      e.preventDefault();
      const nome = document.getElementById('empresa-nome').value.trim();
      const cnpj = document.getElementById('empresa-cnpj').value.trim();
      if (!nome) return;
      try {
        await App.CriarEmpresa(nome, cnpj);
        e.target.reset();
        toast('Empresa "' + nome + '" criada e ativada.');
      } catch (err) {
        toast('Falha ao criar empresa: ' + err, true);
      }
    });

    if (window.runtime && window.runtime.EventsOn) {
      window.runtime.EventsOn('empresa:trocada', () => aoTrocarEmpresa());
    }
  }

  async function carregarEmpresas() {
    state.empresas = await App.ListEmpresas();
    const ativa = await App.EmpresaAtiva();
    state.empresaAtivaId = ativa.id;

    const sel = document.getElementById('empresa-atual');
    sel.innerHTML = state.empresas
      .map((e) => `<option value="${e.id}">${escapeHtml(e.nome)}</option>`)
      .join('');
    sel.value = String(ativa.id);

    const lista = document.getElementById('lista-empresas');
    if (!lista) return;
    lista.innerHTML = state.empresas.map((e) => `
      <div class="mini-row">
        <div class="mini-main">
          <span>${escapeHtml(e.nome)}${e.id === ativa.id ? ' <span class="badge pago">ativa</span>' : ''}</span>
          <span class="mini-sub">${e.cnpj ? escapeHtml(e.cnpj) + ' · ' : ''}desde ${fmtDate(e.criadaEm)}</span>
        </div>
        <span class="row-actions">
          ${e.id === ativa.id ? '' : `<button class="icon-btn" data-emp-usar="${e.id}">Usar</button>`}
          <button class="icon-btn" data-emp-editar="${e.id}">Renomear</button>
          <button class="icon-btn danger" data-emp-excluir="${e.id}">Excluir</button>
        </span>
      </div>`).join('');

    lista.querySelectorAll('[data-emp-usar]').forEach((b) =>
      b.addEventListener('click', () => App.TrocarEmpresa(Number(b.dataset.empUsar))));
    lista.querySelectorAll('[data-emp-editar]').forEach((b) =>
      b.addEventListener('click', () => editarEmpresa(Number(b.dataset.empEditar))));
    lista.querySelectorAll('[data-emp-excluir]').forEach((b) =>
      b.addEventListener('click', () => excluirEmpresa(Number(b.dataset.empExcluir))));
  }

  async function editarEmpresa(id) {
    const emp = state.empresas.find((e) => e.id === id);
    const nome = await askText('Renomear empresa', 'Nome da empresa', emp ? emp.nome : '');
    if (nome === null) return;
    const cnpj = await askText('Renomear empresa', 'CNPJ (opcional)', emp ? emp.cnpj : '');
    if (cnpj === null) return;
    try {
      await App.AtualizarEmpresa(id, nome.trim(), cnpj.trim());
      toast('Empresa atualizada.');
      carregarEmpresas();
    } catch (err) {
      toast('Falha ao atualizar: ' + err, true);
    }
  }

  async function excluirEmpresa(id) {
    const emp = state.empresas.find((e) => e.id === id);
    const ok = await askConfirm(
      'Excluir empresa',
      `Excluir "${emp ? emp.nome : 'esta empresa'}" e TODOS os seus dados (contas, categorias, lançamentos)?`,
      'Excluir'
    );
    if (!ok) return;
    try {
      await App.ExcluirEmpresa(id);
      toast('Empresa excluída.');
      carregarEmpresas();
    } catch (err) {
      toast('' + err, true);
    }
  }

  // recarrega tudo depois de trocar de empresa
  async function aoTrocarEmpresa() {
    await refreshCategoriasEContas();
    await carregarEmpresas();
    goToPage(paginaAtual());
  }

  // ---------------------------------------------------------------
  // Dashboard
  // ---------------------------------------------------------------

  async function loadDashboard() {
    const resumo = await App.DashboardResumo();

    document.getElementById('dash-hoje').textContent =
      'Atualizado em ' + fmtDate(resumo.hoje);
    document.getElementById('card-saldo').textContent = fmtMoney(resumo.saldoAtual);
    document.getElementById('card-areceber').textContent = fmtMoney(resumo.totalAReceber);
    document.getElementById('card-apagar').textContent = fmtMoney(resumo.totalAPagar);

    const wrap = document.getElementById('dash-vencimentos');
    wrap.innerHTML = '';
    const proximos = resumo.proximosVencimentos || [];
    if (proximos.length === 0) {
      wrap.innerHTML = '<div class="empty-msg">Nenhum vencimento em aberto.</div>';
    }
    proximos.forEach((l) => {
      const row = document.createElement('div');
      row.className = 'mini-row';
      row.innerHTML = `
        <div class="mini-main">
          <span>${escapeHtml(l.descricao)}</span>
          <span class="mini-sub">${fmtDate(l.dataVencimento)} · <span class="badge ${l.status}">${statusLabel(l.status)}</span></span>
        </div>
        <span class="mini-value ${l.tipo === 'pagar' ? 'pagar' : 'receber'}">
          ${l.tipo === 'pagar' ? '-' : '+'} ${fmtMoney(l.valor)}
        </span>`;
      wrap.appendChild(row);
    });

    drawBarChart('chart-fluxo', (resumo.fluxoMensal || []).map((m) => ({
      label: monthLabel(m.mes),
      a: m.entradas,
      b: m.saidas
    })));
  }

  function statusLabel(status) {
    return { pendente: 'Pendente', atrasado: 'Atrasado', pago: 'Pago', recebido: 'Recebido' }[status] || status;
  }

  // ---------------------------------------------------------------
  // Gráfico de barras simples (canvas), sem dependências externas
  // ---------------------------------------------------------------

  function drawBarChart(canvasId, series) {
    const canvas = document.getElementById(canvasId);
    const ctx = canvas.getContext('2d');
    const W = canvas.width, H = canvas.height;
    ctx.clearRect(0, 0, W, H);

    if (series.length === 0) return;

    const padding = { top: 16, right: 16, bottom: 28, left: 50 };
    const chartW = W - padding.left - padding.right;
    const chartH = H - padding.top - padding.bottom;

    const maxVal = Math.max(1, ...series.flatMap((s) => [s.a, s.b]));
    const groupW = chartW / series.length;
    const barW = Math.min(28, groupW / 3.2);

    // eixos
    ctx.strokeStyle = '#e2e8f0';
    ctx.beginPath();
    ctx.moveTo(padding.left, padding.top);
    ctx.lineTo(padding.left, H - padding.bottom);
    ctx.lineTo(W - padding.right, H - padding.bottom);
    ctx.stroke();

    // linhas de grade + labels do eixo Y
    ctx.fillStyle = '#94a3b8';
    ctx.font = '10px sans-serif';
    ctx.textAlign = 'right';
    const steps = 4;
    for (let i = 0; i <= steps; i++) {
      const v = (maxVal / steps) * i;
      const y = H - padding.bottom - (chartH * i) / steps;
      ctx.strokeStyle = '#f1f5f9';
      ctx.beginPath();
      ctx.moveTo(padding.left, y);
      ctx.lineTo(W - padding.right, y);
      ctx.stroke();
      ctx.fillText(shortMoney(v), padding.left - 6, y + 3);
    }

    series.forEach((s, i) => {
      const groupX = padding.left + i * groupW;
      const xa = groupX + groupW / 2 - barW - 3;
      const xb = groupX + groupW / 2 + 3;

      const ha = (s.a / maxVal) * chartH;
      const hb = (s.b / maxVal) * chartH;

      ctx.fillStyle = '#16a34a';
      ctx.fillRect(xa, H - padding.bottom - ha, barW, ha);

      ctx.fillStyle = '#dc2626';
      ctx.fillRect(xb, H - padding.bottom - hb, barW, hb);

      ctx.fillStyle = '#64748b';
      ctx.textAlign = 'center';
      ctx.fillText(s.label, groupX + groupW / 2, H - padding.bottom + 14);
    });
  }

  function shortMoney(v) {
    if (v >= 1000) return (v / 1000).toFixed(1).replace('.0', '') + 'k';
    return String(Math.round(v));
  }

  // ---------------------------------------------------------------
  // Contas a pagar / a receber
  // ---------------------------------------------------------------

  function setupFiltros() {
    ['pagar', 'receber'].forEach((tipo) => {
      const wrap = document.querySelector(`.filters[data-filters="${tipo}"]`);
      const opcoes = [['todos', 'Todos'], ['pendente', 'Pendentes'], ['atrasado', 'Atrasados'],
        [tipo === 'pagar' ? 'pago' : 'recebido', tipo === 'pagar' ? 'Pagos' : 'Recebidos']];
      opcoes.forEach(([valor, label]) => {
        const btn = document.createElement('button');
        btn.className = 'filter-btn' + (valor === 'todos' ? ' active' : '');
        btn.textContent = label;
        btn.dataset.valor = valor;
        btn.addEventListener('click', () => {
          wrap.querySelectorAll('.filter-btn').forEach((b) => b.classList.remove('active'));
          btn.classList.add('active');
          if (tipo === 'pagar') state.filtroPagar = valor; else state.filtroReceber = valor;
          loadLancamentos(tipo);
        });
        wrap.appendChild(btn);
      });
    });
  }

  async function loadLancamentos(tipo) {
    const filtroStatus = tipo === 'pagar' ? state.filtroPagar : state.filtroReceber;
    const filtros = { tipo, status: '', dataInicio: '', dataFim: '' };
    if (filtroStatus !== 'todos') filtros.status = filtroStatus;

    const itens = await App.ListLancamentos(filtros);
    const tbody = document.querySelector(`#table-${tipo} tbody`);
    tbody.innerHTML = '';

    if (itens.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="empty-msg">Nenhum lançamento encontrado.</td></tr>';
      return;
    }

    itens.forEach((l) => {
      const categoria = state.categorias.find((c) => c.id === l.categoriaId);
      const tr = document.createElement('tr');
      const jaBaixado = l.status === 'pago' || l.status === 'recebido';
      tr.innerHTML = `
        <td>${escapeHtml(l.descricao)}</td>
        <td>${categoria ? escapeHtml(categoria.nome) : '—'}</td>
        <td>${fmtDate(l.dataVencimento)}</td>
        <td>${fmtMoney(l.valor)}</td>
        <td><span class="badge ${l.status}">${statusLabel(l.status)}</span></td>
        <td class="row-actions">
          ${jaBaixado
            ? `<button class="icon-btn" data-action="estornar" data-id="${l.id}">Estornar</button>`
            : `<button class="icon-btn" data-action="baixar" data-id="${l.id}">${tipo === 'pagar' ? 'Pagar' : 'Receber'}</button>`}
          <button class="icon-btn" data-action="editar" data-id="${l.id}" data-tipo="${tipo}">Editar</button>
          <button class="icon-btn danger" data-action="excluir" data-id="${l.id}">Excluir</button>
        </td>`;
      tbody.appendChild(tr);
    });

    tbody.querySelectorAll('[data-action]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        const id = Number(btn.dataset.id);
        const action = btn.dataset.action;
        if (action === 'baixar') await App.MarcarBaixa(id, '');
        if (action === 'estornar') await App.Estornar(id);
        if (action === 'excluir') {
          if (await askConfirm('Excluir lançamento', 'Excluir este lançamento?', 'Excluir')) {
            await App.DeleteLancamento(id);
          }
        }
        if (action === 'editar') return openModalLancamento(btn.dataset.tipo, id);
        loadLancamentos(tipo);
      });
    });
  }

  // ---------------------------------------------------------------
  // Modal de lançamento (criar/editar)
  // ---------------------------------------------------------------

  function setupModalLancamento() {
    document.querySelectorAll('[data-open-modal="lancamento"]').forEach((btn) => {
      btn.addEventListener('click', () => openModalLancamento(btn.dataset.tipo));
    });
    document.querySelectorAll('[data-close-modal="lancamento"]').forEach((btn) => {
      btn.addEventListener('click', () => closeModal('lancamento'));
    });
    document.getElementById('form-lancamento').addEventListener('submit', onSubmitLancamento);
  }

  async function openModalLancamento(tipo, id = null) {
    const selCategoria = document.getElementById('lanc-categoria');
    const selConta = document.getElementById('lanc-conta');
    selCategoria.innerHTML = state.categorias
      .filter((c) => c.tipo === (tipo === 'pagar' ? 'despesa' : 'receita'))
      .map((c) => `<option value="${c.id}">${escapeHtml(c.nome)}</option>`).join('');
    selConta.innerHTML = state.contas.map((c) => `<option value="${c.id}">${escapeHtml(c.nome)}</option>`).join('');

    document.getElementById('lanc-tipo').value = tipo;
    document.getElementById('modal-titulo').textContent =
      (id ? 'Editar' : 'Nova') + (tipo === 'pagar' ? ' conta a pagar' : ' conta a receber');

    if (id) {
      const itens = await App.ListLancamentos({ tipo, status: '', dataInicio: '', dataFim: '' });
      const l = itens.find((x) => x.id === id);
      document.getElementById('lanc-id').value = l.id;
      document.getElementById('lanc-descricao').value = l.descricao;
      document.getElementById('lanc-categoria').value = l.categoriaId || '';
      document.getElementById('lanc-conta').value = l.contaId || '';
      document.getElementById('lanc-valor').value = l.valor;
      document.getElementById('lanc-vencimento').value = l.dataVencimento;
      document.getElementById('lanc-obs').value = l.observacoes || '';
    } else {
      document.getElementById('form-lancamento').reset();
      document.getElementById('lanc-id').value = '';
      document.getElementById('lanc-tipo').value = tipo;
    }

    openModal('lancamento');
  }

  async function onSubmitLancamento(e) {
    e.preventDefault();
    const id = document.getElementById('lanc-id').value;
    const input = {
      tipo: document.getElementById('lanc-tipo').value,
      descricao: document.getElementById('lanc-descricao').value.trim(),
      categoriaId: Number(document.getElementById('lanc-categoria').value) || null,
      contaId: Number(document.getElementById('lanc-conta').value) || null,
      valor: Number(document.getElementById('lanc-valor').value),
      dataVencimento: document.getElementById('lanc-vencimento').value,
      observacoes: document.getElementById('lanc-obs').value.trim()
    };

    if (id) {
      await App.UpdateLancamento(Number(id), input);
    } else {
      await App.CreateLancamento(input);
    }

    closeModal('lancamento');
    loadLancamentos(input.tipo);
    loadDashboard();
  }

  function openModal(name) { document.getElementById(`modal-${name}`).classList.add('open'); }
  function closeModal(name) { document.getElementById(`modal-${name}`).classList.remove('open'); }

  // Diálogos em DOM — o WKWebView do macOS não implementa window.prompt/confirm,
  // então esses helpers os substituem devolvendo uma Promise.
  function askDialog({ title, message = '', input = false, value = '', label = 'Valor', okText = 'Confirmar', danger = false }) {
    return new Promise((resolve) => {
      const backdrop = document.getElementById('modal-ask');
      const field = document.getElementById('ask-field');
      const inp = document.getElementById('ask-input');
      const okBtn = document.getElementById('ask-ok');
      const cancelBtn = document.getElementById('ask-cancel');
      const msgEl = document.getElementById('ask-message');

      document.getElementById('ask-title').textContent = title;
      msgEl.textContent = message;
      msgEl.style.display = message ? 'block' : 'none';
      document.getElementById('ask-label').textContent = label;
      field.style.display = input ? 'flex' : 'none';
      inp.value = value || '';
      okBtn.textContent = okText;
      okBtn.style.background = danger ? 'var(--red)' : '';

      const done = (result) => {
        backdrop.classList.remove('open');
        okBtn.removeEventListener('click', onOk);
        cancelBtn.removeEventListener('click', onCancel);
        document.removeEventListener('keydown', onKey);
        resolve(result);
      };
      const onOk = () => done(input ? inp.value : true);
      const onCancel = () => done(input ? null : false);
      const onKey = (e) => {
        if (e.key === 'Escape') onCancel();
        else if (e.key === 'Enter' && input) onOk();
      };

      okBtn.addEventListener('click', onOk);
      cancelBtn.addEventListener('click', onCancel);
      document.addEventListener('keydown', onKey);
      backdrop.classList.add('open');
      if (input) setTimeout(() => { inp.focus(); inp.select(); }, 30);
    });
  }

  const askText = (title, label, value) => askDialog({ title, label, value, input: true });
  const askConfirm = (title, message, okText = 'Confirmar') => askDialog({ title, message, okText, danger: true });

  // ---------------------------------------------------------------
  // Fluxo de caixa
  // ---------------------------------------------------------------

  function setupFluxo() {
    const sel = document.getElementById('fluxo-ano');
    const anoAtual = new Date().getFullYear();
    for (let a = anoAtual - 2; a <= anoAtual + 1; a++) {
      const opt = document.createElement('option');
      opt.value = a; opt.textContent = a;
      if (a === anoAtual) opt.selected = true;
      sel.appendChild(opt);
    }
    sel.addEventListener('change', loadFluxo);
  }

  async function loadFluxo() {
    const ano = Number(document.getElementById('fluxo-ano').value);
    const linhas = await App.RelatorioFluxoCaixa(ano);
    const tbody = document.querySelector('#table-fluxo tbody');
    tbody.innerHTML = linhas.map((l) => `
      <tr>
        <td>${monthLabel(l.mes)}</td>
        <td style="color:#16a34a">${fmtMoney(l.entradas)}</td>
        <td style="color:#dc2626">${fmtMoney(l.saidas)}</td>
        <td>${fmtMoney(l.saldoPeriodo)}</td>
        <td><strong>${fmtMoney(l.saldoAcumulado)}</strong></td>
      </tr>`).join('');
  }

  // ---------------------------------------------------------------
  // DRE
  // ---------------------------------------------------------------

  function setupDre() {
    const hoje = new Date();
    const inicio = new Date(hoje.getFullYear(), hoje.getMonth(), 1);
    document.getElementById('dre-inicio').value = isoDate(inicio);
    document.getElementById('dre-fim').value = isoDate(hoje);
    document.getElementById('dre-atualizar').addEventListener('click', loadDre);
  }

  function isoDate(d) {
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  }

  async function loadDre() {
    const dataInicio = document.getElementById('dre-inicio').value;
    const dataFim = document.getElementById('dre-fim').value;
    const dre = await App.RelatorioDRE(dataInicio, dataFim);

    document.getElementById('dre-receita').textContent = fmtMoney(dre.receitaBruta);
    document.getElementById('dre-despesa').textContent = fmtMoney(dre.despesas);
    document.getElementById('dre-resultado').textContent = fmtMoney(dre.resultado);

    const tbody = document.querySelector('#table-dre tbody');
    const linhas = dre.linhas || [];
    if (linhas.length === 0) {
      tbody.innerHTML = '<tr><td colspan="3" class="empty-msg">Nenhum lançamento no período.</td></tr>';
      return;
    }
    tbody.innerHTML = linhas.map((l) => `
      <tr>
        <td>${escapeHtml(l.categoria)}</td>
        <td>${l.tipo === 'receita' ? 'Receita' : 'Despesa'}</td>
        <td style="color:${l.tipo === 'receita' ? '#16a34a' : '#dc2626'}">${fmtMoney(l.total)}</td>
      </tr>`).join('');
  }

  // ---------------------------------------------------------------
  // Cadastros: contas bancárias e categorias
  // ---------------------------------------------------------------

  function setupCadastros() {
    document.getElementById('form-conta').addEventListener('submit', async (e) => {
      e.preventDefault();
      const nome = document.getElementById('conta-nome').value.trim();
      const saldoInicial = Number(document.getElementById('conta-saldo').value) || 0;
      if (!nome) return;
      await App.CreateConta(nome, saldoInicial);
      e.target.reset();
      await refreshCategoriasEContas();
      loadCadastros();
    });

    document.getElementById('form-categoria').addEventListener('submit', async (e) => {
      e.preventDefault();
      const nome = document.getElementById('categoria-nome').value.trim();
      const tipo = document.getElementById('categoria-tipo').value;
      if (!nome) return;
      await App.CreateCategoria(nome, tipo);
      e.target.reset();
      await refreshCategoriasEContas();
      loadCadastros();
    });
  }

  async function loadCadastros() {
    await refreshCategoriasEContas();

    const listaContas = document.getElementById('lista-contas');
    listaContas.innerHTML = state.contas.map((c) => `
      <div class="mini-row">
        <div class="mini-main">
          <span>${escapeHtml(c.nome)}</span>
          <span class="mini-sub">Saldo inicial: ${fmtMoney(c.saldoInicial)}</span>
        </div>
        <button class="mini-remove" data-remove-conta="${c.id}">✕</button>
      </div>`).join('') || '<div class="empty-msg">Nenhuma conta cadastrada.</div>';

    const listaCategorias = document.getElementById('lista-categorias');
    listaCategorias.innerHTML = state.categorias.map((c) => `
      <div class="mini-row">
        <div class="mini-main">
          <span>${escapeHtml(c.nome)}</span>
          <span class="mini-sub">${c.tipo === 'receita' ? 'Receita' : 'Despesa'}</span>
        </div>
        <button class="mini-remove" data-remove-categoria="${c.id}">✕</button>
      </div>`).join('') || '<div class="empty-msg">Nenhuma categoria cadastrada.</div>';

    listaContas.querySelectorAll('[data-remove-conta]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        await App.DeleteConta(Number(btn.dataset.removeConta));
        await refreshCategoriasEContas();
        loadCadastros();
      });
    });
    listaCategorias.querySelectorAll('[data-remove-categoria]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        await App.DeleteCategoria(Number(btn.dataset.removeCategoria));
        await refreshCategoriasEContas();
        loadCadastros();
      });
    });
  }

  async function refreshCategoriasEContas() {
    state.categorias = await App.ListCategorias();
    state.contas = await App.ListContas();
  }

  // ---------------------------------------------------------------
  // Exportação de relatórios (PDF / Excel / CSV)
  // ---------------------------------------------------------------

  function setupExport() {
    document.querySelectorAll('.export').forEach((grupo) => {
      const alvo = grupo.dataset.export;
      grupo.querySelectorAll('.btn-export').forEach((btn) => {
        btn.addEventListener('click', () => exportar(alvo, btn.dataset.fmt, btn));
      });
    });
  }

  async function exportar(alvo, fmt, btn) {
    const grupo = btn.closest('.export');
    grupo.querySelectorAll('.btn-export').forEach((b) => (b.disabled = true));
    const rotulo = btn.textContent;
    btn.textContent = '…';
    try {
      let caminho = '';
      if (alvo === 'fluxo') {
        caminho = await App.ExportarFluxoCaixa(Number(document.getElementById('fluxo-ano').value), fmt);
      } else if (alvo === 'dre') {
        caminho = await App.ExportarDRE(
          document.getElementById('dre-inicio').value,
          document.getElementById('dre-fim').value,
          fmt
        );
      } else if (alvo === 'pagar' || alvo === 'receber') {
        const filtroStatus = alvo === 'pagar' ? state.filtroPagar : state.filtroReceber;
        caminho = await App.ExportarLancamentos(
          { tipo: alvo, status: filtroStatus === 'todos' ? '' : filtroStatus, dataInicio: '', dataFim: '' },
          fmt
        );
      }
      if (caminho) toast('Relatório salvo em ' + caminho);
    } catch (err) {
      toast('Falha ao exportar: ' + err, true);
    } finally {
      btn.textContent = rotulo;
      grupo.querySelectorAll('.btn-export').forEach((b) => (b.disabled = false));
    }
  }

  let toastTimer = null;
  function toast(msg, erro = false) {
    let el = document.getElementById('toast');
    if (!el) {
      el = document.createElement('div');
      el.id = 'toast';
      document.body.appendChild(el);
    }
    el.textContent = msg;
    el.className = 'toast show' + (erro ? ' error' : '');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => (el.className = 'toast'), 5000);
  }

  // ---------------------------------------------------------------
  // Auto-update (eventos vindos do backend Go)
  // ---------------------------------------------------------------

  function setupUpdate() {
    if (!window.runtime || !window.runtime.EventsOn) return;
    window.runtime.EventsOn('update:disponivel', (at) => bannerUpdate('disponivel', at));
    window.runtime.EventsOn('update:baixando', () => bannerUpdate('baixando'));
    window.runtime.EventsOn('update:concluido', (d) => bannerUpdate('concluido', d));
    window.runtime.EventsOn('update:erro', (msg) => {
      bannerUpdate('disponivel', updateState.at);
      toast('Falha na atualização: ' + msg, true);
    });
  }

  const updateState = { at: null };

  function bannerEl() {
    let el = document.getElementById('update-banner');
    if (!el) {
      el = document.createElement('div');
      el.id = 'update-banner';
      el.className = 'update-banner';
      document.body.appendChild(el);
    }
    return el;
  }

  function bannerUpdate(estado, dados) {
    const el = bannerEl();

    if (estado === 'disponivel') {
      updateState.at = dados;
      el.innerHTML = `
        <span class="update-msg">Fynam <strong>${escapeHtml(dados.versaoNova)}</strong> disponível
          <span class="update-sub">(você está na ${escapeHtml(dados.versaoAtual)})</span></span>
        <span class="update-actions">
          <button class="btn btn-ghost" data-upd="notas">Ver notas</button>
          <button class="btn btn-primary" data-upd="aplicar">Atualizar agora</button>
          <button class="btn btn-ghost" data-upd="depois">Depois</button>
        </span>`;
      el.className = 'update-banner show';
    } else if (estado === 'baixando') {
      el.innerHTML = `<span class="update-msg">Baixando e instalando a atualização…</span>`;
      el.className = 'update-banner show';
    } else if (estado === 'concluido') {
      el.innerHTML = `
        <span class="update-msg">Atualização instalada.</span>
        <span class="update-actions">
          <button class="btn btn-primary" data-upd="reiniciar">Reiniciar agora</button>
          <button class="btn btn-ghost" data-upd="depois">Depois</button>
        </span>`;
      el.className = 'update-banner show';
    }

    el.querySelectorAll('[data-upd]').forEach((btn) => {
      btn.addEventListener('click', () => acaoUpdate(btn.dataset.upd));
    });
  }

  async function acaoUpdate(acao) {
    if (acao === 'notas' && updateState.at) {
      window.runtime.BrowserOpenURL(updateState.at.url);
      return;
    }
    if (acao === 'depois') {
      bannerEl().className = 'update-banner';
      return;
    }
    if (acao === 'aplicar') {
      try {
        await App.BaixarEAplicarAtualizacao();
      } catch (err) {
        toast('Falha na atualização: ' + err, true);
      }
      return;
    }
    if (acao === 'reiniciar') {
      try {
        await App.ReiniciarApp();
      } catch (err) {
        toast('Não foi possível reiniciar: ' + err, true);
      }
    }
  }

  // ---------------------------------------------------------------
  // Utilidades
  // ---------------------------------------------------------------

  function escapeHtml(str) {
    return String(str ?? '').replace(/[&<>"']/g, (c) => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
    }[c]));
  }

  // ---------------------------------------------------------------
  // Boot
  // ---------------------------------------------------------------

  async function mostrarVersao() {
    const el = document.getElementById('sidebar-versao');
    if (!el) return;
    try {
      const v = await App.VersaoAtual();
      if (v) el.textContent = 'v' + v;
    } catch (_) { /* mantém o texto padrão */ }
  }

  async function boot() {
    setupNav();
    setupFiltros();
    setupModalLancamento();
    setupFluxo();
    setupDre();
    setupCadastros();
    setupExport();
    setupUpdate();
    setupEmpresas();
    mostrarVersao();
    await carregarEmpresas();
    await refreshCategoriasEContas();
    await loadDashboard();
  }

  document.addEventListener('DOMContentLoaded', boot);
})();
