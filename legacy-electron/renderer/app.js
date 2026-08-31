// app.js
// Lógica da interface (processo renderer). Fala com o processo main
// somente através de window.xfin, exposto pelo preload.js.

(() => {
  const state = {
    categorias: [],
    contas: [],
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
    if (page === 'cadastros') loadCadastros();
  }

  // ---------------------------------------------------------------
  // Dashboard
  // ---------------------------------------------------------------

  async function loadDashboard() {
    const resumo = await window.xfin.dashboard.resumo();

    document.getElementById('dash-hoje').textContent =
      'Atualizado em ' + fmtDate(resumo.hoje);
    document.getElementById('card-saldo').textContent = fmtMoney(resumo.saldoAtual);
    document.getElementById('card-areceber').textContent = fmtMoney(resumo.totalAReceber);
    document.getElementById('card-apagar').textContent = fmtMoney(resumo.totalAPagar);

    const wrap = document.getElementById('dash-vencimentos');
    wrap.innerHTML = '';
    if (resumo.proximosVencimentos.length === 0) {
      wrap.innerHTML = '<div class="empty-msg">Nenhum vencimento em aberto.</div>';
    }
    resumo.proximosVencimentos.forEach((l) => {
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

    drawBarChart('chart-fluxo', resumo.fluxoMensal.map((m) => ({
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
    const filtros = { tipo };
    if (filtroStatus !== 'todos') filtros.status = filtroStatus;

    const itens = await window.xfin.lancamentos.list(filtros);
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
        if (action === 'baixar') await window.xfin.lancamentos.marcarBaixa({ id });
        if (action === 'estornar') await window.xfin.lancamentos.estornar(id);
        if (action === 'excluir') {
          if (confirm('Excluir este lançamento?')) await window.xfin.lancamentos.delete(id);
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
      const itens = await window.xfin.lancamentos.list({ tipo });
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
    const payload = {
      tipo: document.getElementById('lanc-tipo').value,
      descricao: document.getElementById('lanc-descricao').value.trim(),
      categoriaId: Number(document.getElementById('lanc-categoria').value) || null,
      contaId: Number(document.getElementById('lanc-conta').value) || null,
      valor: Number(document.getElementById('lanc-valor').value),
      dataVencimento: document.getElementById('lanc-vencimento').value,
      observacoes: document.getElementById('lanc-obs').value.trim()
    };

    if (id) {
      await window.xfin.lancamentos.update({ id: Number(id), ...payload });
    } else {
      await window.xfin.lancamentos.create(payload);
    }

    closeModal('lancamento');
    loadLancamentos(payload.tipo);
    loadDashboard();
  }

  function openModal(name) { document.getElementById(`modal-${name}`).classList.add('open'); }
  function closeModal(name) { document.getElementById(`modal-${name}`).classList.remove('open'); }

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
    const linhas = await window.xfin.relatorios.fluxoCaixa({ ano });
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

  function isoDate(d) { return d.toISOString().slice(0, 10); }

  async function loadDre() {
    const dataInicio = document.getElementById('dre-inicio').value;
    const dataFim = document.getElementById('dre-fim').value;
    const dre = await window.xfin.relatorios.dre({ dataInicio, dataFim });

    document.getElementById('dre-receita').textContent = fmtMoney(dre.receitaBruta);
    document.getElementById('dre-despesa').textContent = fmtMoney(dre.despesas);
    document.getElementById('dre-resultado').textContent = fmtMoney(dre.resultado);

    const tbody = document.querySelector('#table-dre tbody');
    if (dre.linhas.length === 0) {
      tbody.innerHTML = '<tr><td colspan="3" class="empty-msg">Nenhum lançamento no período.</td></tr>';
      return;
    }
    tbody.innerHTML = dre.linhas.map((l) => `
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
      await window.xfin.contas.create({ nome, saldoInicial });
      e.target.reset();
      await refreshCategoriasEContas();
      loadCadastros();
    });

    document.getElementById('form-categoria').addEventListener('submit', async (e) => {
      e.preventDefault();
      const nome = document.getElementById('categoria-nome').value.trim();
      const tipo = document.getElementById('categoria-tipo').value;
      if (!nome) return;
      await window.xfin.categorias.create({ nome, tipo });
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
        await window.xfin.contas.delete(Number(btn.dataset.removeConta));
        await refreshCategoriasEContas();
        loadCadastros();
      });
    });
    listaCategorias.querySelectorAll('[data-remove-categoria]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        await window.xfin.categorias.delete(Number(btn.dataset.removeCategoria));
        await refreshCategoriasEContas();
        loadCadastros();
      });
    });
  }

  async function refreshCategoriasEContas() {
    state.categorias = await window.xfin.categorias.list();
    state.contas = await window.xfin.contas.list();
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

  async function boot() {
    setupNav();
    setupFiltros();
    setupModalLancamento();
    setupFluxo();
    setupDre();
    setupCadastros();
    await refreshCategoriasEContas();
    await loadDashboard();
  }

  document.addEventListener('DOMContentLoaded', boot);
})();
