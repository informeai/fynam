// app.js
// Lógica da interface. Fala com o backend Go somente através dos métodos
// que o Wails expõe em window.go.main.App.*

(() => {
  const App = window.go.main.App;

  function filtroVazio() {
    return {
      status: 'todos', busca: '', categoriaId: '', contaId: '',
      vencDe: '', vencAte: '', valorMin: '', valorMax: ''
    };
  }

  const state = {
    categorias: [],
    contas: [],
    empresas: [],
    empresaAtivaId: null,
    filtros: { pagar: filtroVazio(), receber: filtroVazio() }
  };

  let ddEmpresa = null;
  let ddPeriodo = null;

  // Dropdown genérico: abre/fecha, fecha ao clicar fora ou com Esc, e só um
  // aberto por vez. Devolve { open, close, trigger, menu }.
  function initDropdown(root) {
    const trigger = root.querySelector('.dd-trigger');
    const menu = root.querySelector('.dd-menu');
    const close = () => {
      root.dataset.open = 'false';
      menu.hidden = true;
      trigger.setAttribute('aria-expanded', 'false');
    };
    const open = () => {
      document.querySelectorAll('.dd[data-open="true"]').forEach((d) => {
        if (d !== root && d.__dd) d.__dd.close();
      });
      root.dataset.open = 'true';
      menu.hidden = false;
      trigger.setAttribute('aria-expanded', 'true');
    };
    trigger.addEventListener('click', (e) => {
      e.stopPropagation();
      root.dataset.open === 'true' ? close() : open();
    });
    menu.addEventListener('click', (e) => e.stopPropagation());
    document.addEventListener('click', close);
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape') close(); });
    close();
    root.__dd = { open, close, trigger, menu };
    return root.__dd;
  }

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

  // grupo pai -> páginas dos submenus
  const NAV_GRUPOS = {
    movimentacoes: ['pagar', 'receber', 'fluxo'],
    relatorios: ['dre'],
    cadastros: ['empresas', 'categorias', 'contas']
  };

  function setupNav() {
    document.querySelectorAll('.nav-parent').forEach((btn) => {
      btn.addEventListener('click', () => alternarGrupo(btn.closest('.nav-group')));
    });
    document.querySelectorAll('.nav-item[data-page]').forEach((btn) => {
      btn.addEventListener('click', () => goToPage(btn.dataset.page));
    });
  }

  function setGrupoAberto(grupo, aberto) {
    grupo.querySelector('.nav-sub').hidden = !aberto;
    grupo.querySelector('.nav-parent').setAttribute('aria-expanded', String(aberto));
  }

  // accordion: abrir um grupo fecha os demais
  function alternarGrupo(grupo) {
    const abrir = grupo.querySelector('.nav-sub').hidden;
    document.querySelectorAll('.nav-group').forEach((g) => setGrupoAberto(g, false));
    if (abrir) setGrupoAberto(grupo, true);
  }

  function goToPage(page) {
    document.querySelectorAll('.nav-item[data-page]').forEach((b) =>
      b.classList.toggle('active', b.dataset.page === page));
    document.querySelectorAll('.page').forEach((p) =>
      p.classList.toggle('active', p.id === `page-${page}`));

    const grupoAtivo = Object.keys(NAV_GRUPOS).find((g) => NAV_GRUPOS[g].includes(page));
    document.querySelectorAll('.nav-group').forEach((g) => {
      const ehAtivo = g.dataset.group === grupoAtivo;
      g.querySelector('.nav-parent').classList.toggle('ativo', ehAtivo);
      setGrupoAberto(g, ehAtivo);
    });

    if (page === 'dashboard') loadDashboard();
    if (page === 'pagar') loadLancamentos('pagar');
    if (page === 'receber') loadLancamentos('receber');
    if (page === 'fluxo') loadFluxo();
    if (page === 'dre') loadDre();
    if (page === 'empresas') carregarEmpresas();
    if (page === 'categorias' || page === 'contas') loadCadastros();
  }

  function paginaAtual() {
    const ativa = document.querySelector('.nav-item[data-page].active');
    return ativa ? ativa.dataset.page : 'dashboard';
  }

  // ---------------------------------------------------------------
  // Empresas (múltiplas empresas / filiais)
  // ---------------------------------------------------------------

  function setupEmpresas() {
    ddEmpresa = initDropdown(document.getElementById('dd-empresa'));

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

    const ativaNome = (state.empresas.find((e) => e.id === ativa.id) || {}).nome;
    document.getElementById('empresa-value').textContent = ativaNome || '—';

    const menu = document.getElementById('empresa-menu');
    menu.innerHTML = state.empresas.map((e) => `
      <button type="button" class="dd-item" role="option" data-id="${e.id}" aria-selected="${e.id === ativa.id}">
        <span class="dd-name">${escapeHtml(e.nome)}</span>
        <svg class="dd-check" viewBox="0 0 24 24" aria-hidden="true"><path d="M9 16.2 4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4z"/></svg>
      </button>`).join('');
    menu.querySelectorAll('.dd-item').forEach((it) => {
      it.addEventListener('click', async () => {
        ddEmpresa.close();
        const id = Number(it.dataset.id);
        if (id === state.empresaAtivaId) return;
        try {
          await App.TrocarEmpresa(id);
        } catch (err) {
          toast('Não foi possível trocar de empresa: ' + err, true);
        }
      });
    });

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
    return {
      pendente: 'Pendente', atrasado: 'Atrasado', pago: 'Pago',
      recebido: 'Recebido', conciliado: 'Conciliado'
    }[status] || status;
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
        [tipo === 'pagar' ? 'pago' : 'recebido', tipo === 'pagar' ? 'Pagos' : 'Recebidos'],
        ['conciliado', 'Conciliados']];
      opcoes.forEach(([valor, label]) => {
        const btn = document.createElement('button');
        btn.className = 'filter-btn' + (valor === 'todos' ? ' active' : '');
        btn.textContent = label;
        btn.dataset.valor = valor;
        btn.addEventListener('click', () => {
          wrap.querySelectorAll('.filter-btn').forEach((b) => b.classList.remove('active'));
          btn.classList.add('active');
          state.filtros[tipo].status = valor;
          loadLancamentos(tipo);
        });
        wrap.appendChild(btn);
      });
      atualizarBotaoFiltro(tipo);
    });
  }

  // ---------------------------------------------------------------
  // Modal de filtros por campo (aberto pelo ícone ao lado de "Exportar")
  // ---------------------------------------------------------------

  const camposFiltro = ['busca', 'categoriaId', 'contaId', 'vencDe', 'vencAte', 'valorMin', 'valorMax'];
  let filtroModalTipo = null;
  let filtroDraft = null;

  function temFiltroAtivo(tipo) {
    const f = state.filtros[tipo];
    return camposFiltro.some((k) => f[k] !== '');
  }

  // atualizarBotaoFiltro marca o ícone de filtro quando há filtro por campo
  // aplicado, para ficar visível mesmo com o modal fechado.
  function atualizarBotaoFiltro(tipo) {
    const btn = document.querySelector(`[data-open-filtros="${tipo}"]`);
    if (btn) btn.classList.toggle('tem-filtro', temFiltroAtivo(tipo));
  }

  function setupModalFiltros() {
    document.querySelectorAll('[data-open-filtros]').forEach((btn) => {
      btn.addEventListener('click', () => abrirModalFiltros(btn.dataset.openFiltros));
    });
    document.querySelectorAll('[data-close-modal="filtros"]').forEach((btn) => {
      btn.addEventListener('click', () => closeModal('filtros'));
    });
    document.getElementById('filtros-limpar').addEventListener('click', () => {
      filtroDraft = { status: filtroDraft.status };
      camposFiltro.forEach((k) => { filtroDraft[k] = ''; });
      preencherModalFiltros();
      state.filtros[filtroModalTipo] = { ...filtroDraft };
      atualizarBotaoFiltro(filtroModalTipo);
      loadLancamentos(filtroModalTipo);
    });
    document.getElementById('filtros-aplicar').addEventListener('click', () => {
      state.filtros[filtroModalTipo] = { ...filtroDraft };
      atualizarBotaoFiltro(filtroModalTipo);
      loadLancamentos(filtroModalTipo);
      closeModal('filtros');
    });
  }

  function abrirModalFiltros(tipo) {
    filtroModalTipo = tipo;
    filtroDraft = { ...state.filtros[tipo] };

    const catTipo = tipo === 'pagar' ? 'despesa' : 'receita';
    const cats = state.categorias.filter((c) => c.tipo === catTipo);
    document.getElementById('filtros-titulo').textContent =
      'Filtrar ' + (tipo === 'pagar' ? 'contas a pagar' : 'contas a receber');
    document.getElementById('filtros-categoria').innerHTML =
      '<option value="">Todas</option>' +
      cats.map((c) => `<option value="${c.id}">${escapeHtml(c.nome)}</option>`).join('');
    document.getElementById('filtros-conta').innerHTML =
      '<option value="">Todas</option>' +
      state.contas.map((c) => `<option value="${c.id}">${escapeHtml(c.nome)}</option>`).join('');

    preencherModalFiltros();
    openModal('filtros');
  }

  function preencherModalFiltros() {
    document.querySelectorAll('#modal-filtros [data-ff]').forEach((el) => {
      el.value = filtroDraft[el.dataset.ff] || '';
      el.oninput = el.onchange = () => { filtroDraft[el.dataset.ff] = el.value; };
    });
  }

  function filtroPayload(tipo) {
    const f = state.filtros[tipo];
    return {
      tipo,
      status: f.status === 'todos' ? '' : f.status,
      dataInicio: f.vencDe || '',
      dataFim: f.vencAte || '',
      busca: f.busca.trim(),
      categoriaId: f.categoriaId ? Number(f.categoriaId) : null,
      contaId: f.contaId ? Number(f.contaId) : null,
      valorMin: f.valorMin !== '' ? Number(f.valorMin) : null,
      valorMax: f.valorMax !== '' ? Number(f.valorMax) : null
    };
  }

  async function loadLancamentos(tipo) {
    const itens = await App.ListLancamentos(filtroPayload(tipo));
    const tbody = document.querySelector(`#table-${tipo} tbody`);
    tbody.innerHTML = '';

    if (itens.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty-msg">Nenhum lançamento encontrado.</td></tr>';
      return;
    }

    itens.forEach((l) => {
      const categoria = state.categorias.find((c) => c.id === l.categoriaId);
      const tr = document.createElement('tr');
      const conciliado = l.status === 'conciliado';
      const jaBaixado = conciliado || l.status === 'pago' || l.status === 'recebido';
      let acoesStatus;
      if (conciliado) {
        acoesStatus = `<button class="icon-btn" data-action="desconciliar" data-id="${l.id}">Desconciliar</button>`;
      } else if (jaBaixado) {
        acoesStatus =
          `<button class="icon-btn" data-action="conciliar" data-id="${l.id}">Conciliar</button>` +
          `<button class="icon-btn" data-action="estornar" data-id="${l.id}">Estornar</button>`;
      } else {
        acoesStatus = `<button class="icon-btn" data-action="baixar" data-id="${l.id}">${tipo === 'pagar' ? 'Pagar' : 'Receber'}</button>`;
      }
      tr.innerHTML = `
        <td>${escapeHtml(l.descricao)}</td>
        <td>${categoria ? escapeHtml(categoria.nome) : '—'}</td>
        <td>${fmtDate(l.dataVencimento)}</td>
        <td>${l.dataPagamento ? fmtDate(l.dataPagamento) : '—'}</td>
        <td>${fmtMoney(l.valor)}</td>
        <td><span class="badge ${l.status}">${statusLabel(l.status)}</span></td>
        <td class="row-actions">
          ${acoesStatus}
          <button class="icon-btn" data-action="editar" data-id="${l.id}" data-tipo="${tipo}">Editar</button>
          <button class="icon-btn danger" data-action="excluir" data-id="${l.id}">Excluir</button>
        </td>`;
      tbody.appendChild(tr);
    });

    tbody.querySelectorAll('[data-action]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        const id = Number(btn.dataset.id);
        const action = btn.dataset.action;
        if (action === 'baixar') {
          const label = tipo === 'pagar' ? 'Data de pagamento' : 'Data de recebimento';
          const titulo = tipo === 'pagar' ? 'Registrar pagamento' : 'Registrar recebimento';
          const data = await askDate(titulo, label, hojeISO());
          if (data === null) return;
          await App.MarcarBaixa(id, data);
        }
        if (action === 'estornar') await App.Estornar(id);
        if (action === 'conciliar') await App.Conciliar(id, '');
        if (action === 'desconciliar') await App.DesfazerConciliacao(id);
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
    if (!id && state.contas.length === 0) {
      toast('Cadastre uma conta / caixa antes de lançar.', true);
      return;
    }
    const selCategoria = document.getElementById('lanc-categoria');
    const selConta = document.getElementById('lanc-conta');
    selCategoria.innerHTML = state.categorias
      .filter((c) => c.tipo === (tipo === 'pagar' ? 'despesa' : 'receita'))
      .map((c) => `<option value="${c.id}">${escapeHtml(c.nome)}</option>`).join('');
    selConta.innerHTML = state.contas.map((c) => `<option value="${c.id}">${escapeHtml(c.nome)}</option>`).join('');

    document.getElementById('lanc-tipo').value = tipo;
    document.getElementById('modal-titulo').textContent =
      (id ? 'Editar' : 'Nova') + (tipo === 'pagar' ? ' conta a pagar' : ' conta a receber');
    document.getElementById('lanc-pagamento-label').textContent =
      tipo === 'pagar' ? 'Data de pagamento' : 'Data de recebimento';

    if (id) {
      const itens = await App.ListLancamentos({ tipo, status: '', dataInicio: '', dataFim: '' });
      const l = itens.find((x) => x.id === id);
      document.getElementById('lanc-id').value = l.id;
      document.getElementById('lanc-descricao').value = l.descricao;
      document.getElementById('lanc-categoria').value = l.categoriaId || '';
      document.getElementById('lanc-conta').value = l.contaId || '';
      document.getElementById('lanc-valor').value = l.valor;
      document.getElementById('lanc-vencimento').value = l.dataVencimento;
      document.getElementById('lanc-pagamento').value = l.dataPagamento || '';
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
      dataPagamento: document.getElementById('lanc-pagamento').value,
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

  // ---------------------------------------------------------------
  // Importação e conciliação de extrato OFX
  // ---------------------------------------------------------------

  let ofxPrevia = null;

  // ícone de "upload" para o dropdown Importar
  const IMPORT_ICO = 'M9 16h6v-6h4l-7-7-7 7h4v6zm-4 2h14v2H5v-2z';

  function setupImportacaoOFX() {
    document.querySelectorAll('.dd.import').forEach((root) => {
      root.innerHTML = `
        <button type="button" class="dd-trigger" aria-haspopup="menu" aria-expanded="false">
          <svg class="dd-lead" viewBox="0 0 24 24" aria-hidden="true"><path d="${IMPORT_ICO}"/></svg>
          <span class="dd-txt">Importar</span>
          <svg class="dd-caret" viewBox="0 0 24 24" aria-hidden="true"><path d="M7 10l5 5 5-5z"/></svg>
        </button>
        <div class="dd-menu right" role="menu" hidden>
          <button type="button" class="dd-item" role="menuitem" data-import-tipo="ofx">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M14 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8l-6-6zm-1 7V3.5L18.5 9H13z"/></svg>OFX
          </button>
        </div>`;
      const dd = initDropdown(root);
      root.querySelectorAll('.dd-item').forEach((it) => {
        it.addEventListener('click', () => {
          dd.close();
          if (it.dataset.importTipo === 'ofx') openModalOFX();
        });
      });
    });

    document.querySelectorAll('[data-close-modal="ofx"]').forEach((btn) => {
      btn.addEventListener('click', () => closeModal('ofx'));
    });
    document.getElementById('ofx-escolher').addEventListener('click', escolherArquivoOFX);
    document.getElementById('ofx-aplicar').addEventListener('click', aplicarOFX);
  }

  function openModalOFX() {
    if (state.contas.length === 0) {
      toast('Cadastre uma conta / caixa antes de importar.', true);
      return;
    }
    ofxPrevia = null;
    document.getElementById('ofx-conta').innerHTML =
      state.contas.map((c) => `<option value="${c.id}">${escapeHtml(c.nome)}</option>`).join('');
    document.getElementById('ofx-step-1').hidden = false;
    document.getElementById('ofx-step-2').hidden = true;
    document.getElementById('ofx-linhas').innerHTML = '';
    openModal('ofx');
  }

  async function escolherArquivoOFX() {
    const contaId = Number(document.getElementById('ofx-conta').value);
    let previa;
    try {
      previa = await App.ImportarExtratoOFX(contaId);
    } catch (err) {
      toast(String(err && err.message ? err.message : err), true);
      return;
    }
    if (!previa || !previa.arquivo) return; // usuário cancelou o diálogo
    ofxPrevia = previa;
    renderPreviaOFX(previa);
  }

  function renderPreviaOFX(p) {
    document.getElementById('ofx-step-1').hidden = true;
    document.getElementById('ofx-step-2').hidden = false;
    document.getElementById('ofx-resumo').textContent =
      `${p.arquivo} — ${p.linhas.length} transação(ões)` + (p.periodo ? ` · período ${p.periodo}` : '');

    const aviso = document.getElementById('ofx-aviso');
    aviso.hidden = !p.aviso;
    aviso.textContent = p.aviso || '';

    const tbody = document.getElementById('ofx-linhas');
    tbody.innerHTML = p.linhas.map((lc, i) => {
      const l = lc.linha;
      const mov = l.tipo === 'pagar'
        ? '<span class="badge atrasado">Débito</span>'
        : '<span class="badge recebido">Crédito</span>';
      let acao;
      if (lc.jaImportada) {
        acao = '<span class="mini-sub">Já importada</span>';
      } else {
        const opts = (lc.candidatos || []).map((c) => {
          const sel = lc.sugestao === 'conciliar' && lc.sugestaoId === c.id ? ' selected' : '';
          return `<option value="conciliar:${c.id}"${sel}>Conciliar: ${escapeHtml(c.descricao)} · ${fmtDate(c.dataVencimento)} · ${fmtMoney(c.valor)}</option>`;
        });
        opts.push(`<option value="criar"${lc.sugestao === 'criar' ? ' selected' : ''}>Criar lançamento</option>`);
        opts.push(`<option value="ignorar"${lc.sugestao === 'ignorar' ? ' selected' : ''}>Ignorar</option>`);
        acao = `<select data-i="${i}">${opts.join('')}</select>`;
      }
      return `<tr class="${lc.jaImportada ? 'ofx-feita' : ''}">
        <td>${fmtDate(l.data)}</td>
        <td>${escapeHtml(l.descricao || '—')}</td>
        <td>${mov}</td>
        <td>${fmtMoney(l.valor)}</td>
        <td>${acao}</td>
      </tr>`;
    }).join('');
  }

  async function aplicarOFX() {
    if (!ofxPrevia) return;
    const decisoes = [];
    document.querySelectorAll('#ofx-linhas select[data-i]').forEach((sel) => {
      const lc = ofxPrevia.linhas[Number(sel.dataset.i)];
      const [acao, id] = sel.value.split(':');
      decisoes.push({
        linha: lc.linha,
        acao,
        lancamentoId: acao === 'conciliar' ? Number(id) : null,
        categoriaId: null,
      });
    });

    let r;
    try {
      r = await App.AplicarImportacaoOFX(ofxPrevia.contaId, decisoes);
    } catch (err) {
      toast(String(err && err.message ? err.message : err), true);
      return;
    }

    const temErro = r.erros && r.erros.length > 0;
    toast(`Extrato: ${r.conciliados} conciliado(s), ${r.criados} criado(s), ${r.ignorados} ignorado(s)`
      + (temErro ? ` · ${r.erros.length} erro(s)` : ''), temErro);

    closeModal('ofx');
    ofxPrevia = null;
    await refreshCategoriasEContas();
    loadLancamentos('pagar');
    loadLancamentos('receber');
    loadDashboard();
  }

  // Diálogos em DOM — o WKWebView do macOS não implementa window.prompt/confirm,
  // então esses helpers os substituem devolvendo uma Promise.
  function askDialog({ title, message = '', input = false, inputType = 'text', value = '', label = 'Valor', okText = 'Confirmar', danger = false }) {
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
      inp.type = inputType;
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
      if (input) setTimeout(() => {
        inp.focus();
        if (inp.type === 'text') inp.select();
      }, 30);
    });
  }

  const askText = (title, label, value) => askDialog({ title, label, value, input: true });
  const askDate = (title, label, value) => askDialog({ title, label, value, input: true, inputType: 'date' });
  const askConfirm = (title, message, okText = 'Confirmar') => askDialog({ title, message, okText, danger: true });

  function hojeISO() {
    const d = new Date();
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  }

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

  // Presets do seletor de período da DRE. range() -> [inicio: Date, fim: Date].
  const DRE_PRESETS = {
    'mes': {
      label: 'Este mês',
      range: () => { const h = new Date(); return [new Date(h.getFullYear(), h.getMonth(), 1), h]; }
    },
    'mes-1': {
      label: 'Mês passado',
      range: () => {
        const h = new Date();
        return [new Date(h.getFullYear(), h.getMonth() - 1, 1), new Date(h.getFullYear(), h.getMonth(), 0)];
      }
    },
    '3m': {
      label: 'Últimos 3 meses',
      range: () => { const h = new Date(); return [new Date(h.getFullYear(), h.getMonth() - 2, 1), h]; }
    },
    'ano': {
      label: 'Este ano',
      range: () => { const h = new Date(); return [new Date(h.getFullYear(), 0, 1), h]; }
    }
  };

  function setDrePeriodo(dataInicio, dataFim, rotulo) {
    document.getElementById('dre-inicio').value = dataInicio;
    document.getElementById('dre-fim').value = dataFim;
    document.getElementById('periodo-label').textContent = rotulo;
  }

  function aplicarPreset(chave) {
    const p = DRE_PRESETS[chave];
    const [ini, fim] = p.range();
    setDrePeriodo(isoDate(ini), isoDate(fim), p.label);
  }

  function setupDre() {
    ddPeriodo = initDropdown(document.getElementById('dd-periodo'));

    document.querySelectorAll('#dd-periodo [data-preset]').forEach((b) => {
      b.addEventListener('click', () => {
        aplicarPreset(b.dataset.preset);
        ddPeriodo.close();
        loadDre();
      });
    });

    document.getElementById('dre-aplicar').addEventListener('click', () => {
      const ini = document.getElementById('dre-inicio').value;
      const fim = document.getElementById('dre-fim').value;
      if (!ini || !fim) { toast('Informe as duas datas do intervalo.', true); return; }
      document.getElementById('periodo-label').textContent = `${fmtDate(ini)} – ${fmtDate(fim)}`;
      ddPeriodo.close();
      loadDre();
    });

    aplicarPreset('mes');
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
      const bankId = document.getElementById('conta-bank-id').value.trim();
      const acctId = document.getElementById('conta-acct-id').value.trim();
      const acctType = document.getElementById('conta-acct-type').value;
      if (!nome) return;
      await App.CreateConta(nome, saldoInicial, bankId, acctId, acctType);
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

    const acctTypeLabel = { CHECKING: 'Corrente', SAVINGS: 'Poupança', CASH: 'Caixa' };
    const listaContas = document.getElementById('lista-contas');
    listaContas.innerHTML = state.contas.map((c) => {
      const banco = [c.bankId, c.acctId].filter(Boolean).join(' / ');
      const dados = [`Saldo inicial: ${fmtMoney(c.saldoInicial)}`];
      if (banco) dados.push(`Banco ${escapeHtml(banco)}`);
      if (c.acctType) dados.push(acctTypeLabel[c.acctType] || escapeHtml(c.acctType));
      return `
      <div class="mini-row">
        <div class="mini-main">
          <span>${escapeHtml(c.nome)}</span>
          <span class="mini-sub">${dados.join(' · ')}</span>
        </div>
        <button class="mini-remove" data-remove-conta="${c.id}">✕</button>
      </div>`;
    }).join('') || '<div class="empty-msg">Nenhuma conta cadastrada.</div>';

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
        try {
          await App.DeleteConta(Number(btn.dataset.removeConta));
        } catch (err) {
          toast(String(err && err.message ? err.message : err), true);
          return;
        }
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

  const EXPORT_ICO = 'M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z';
  const EXPORT_FMTS = [
    { fmt: 'pdf', nome: 'PDF', ico: 'M6 2c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8l-6-6H6zm7 7V3.5L18.5 9H13z' },
    { fmt: 'xlsx', nome: 'Excel', ico: 'M4 4h16v16H4V4zm2 2v3h5V6H6zm7 0v3h5V6h-5zm-7 5v3h5v-3H6zm7 0v3h5v-3h-5zm-7 5v2h5v-2H6zm7 0v2h5v-2h-5z' },
    { fmt: 'csv', nome: 'CSV', ico: 'M14 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z' }
  ];

  function setupExport() {
    document.querySelectorAll('.export').forEach((root) => {
      const alvo = root.dataset.export;
      root.innerHTML = `
        <button type="button" class="dd-trigger" aria-haspopup="menu" aria-expanded="false">
          <svg class="dd-lead" viewBox="0 0 24 24" aria-hidden="true"><path d="${EXPORT_ICO}"/></svg>
          <span class="dd-txt">Exportar</span>
          <svg class="dd-caret" viewBox="0 0 24 24" aria-hidden="true"><path d="M7 10l5 5 5-5z"/></svg>
        </button>
        <div class="dd-menu right" role="menu" hidden>
          ${EXPORT_FMTS.map((f) => `
            <button type="button" class="dd-item" role="menuitem" data-fmt="${f.fmt}">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="${f.ico}"/></svg>${f.nome}
            </button>`).join('')}
        </div>`;
      const dd = initDropdown(root);
      root.querySelectorAll('.dd-item').forEach((it) => {
        it.addEventListener('click', () => {
          dd.close();
          exportar(alvo, it.dataset.fmt, root);
        });
      });
    });
  }

  async function exportar(alvo, fmt, root) {
    const trigger = root.querySelector('.dd-trigger');
    const txt = root.querySelector('.dd-txt');
    const rotulo = txt.textContent;
    trigger.disabled = true;
    txt.textContent = 'Exportando…';
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
        caminho = await App.ExportarLancamentos(filtroPayload(alvo), fmt);
      }
      if (caminho) toast('Relatório salvo em ' + caminho);
    } catch (err) {
      toast('Falha ao exportar: ' + err, true);
    } finally {
      txt.textContent = rotulo;
      trigger.disabled = false;
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
    setupImportacaoOFX();
    setupModalFiltros();
    mostrarVersao();
    await carregarEmpresas();
    await refreshCategoriasEContas();
    await loadDashboard();
  }

  document.addEventListener('DOMContentLoaded', boot);
})();
