package main

import (
	"context"
	"log"
	"sort"
	"strconv"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"fynam/internal/model"
	"fynam/internal/storage"
	"fynam/internal/updater"
)

// App reúne todos os métodos expostos ao frontend pelo Wails.
//
// Não conhece SQLite nem nenhum banco específico: fala apenas com a
// interface storage.Store. Toda regra de negócio (status derivado,
// agregações do dashboard e dos relatórios) vive aqui, de forma
// independente do backend de persistência.
type App struct {
	ctx   context.Context
	store storage.Store

	upd               *updater.Updater
	mu                sync.Mutex
	ultimaAtualizacao *updater.Atualizacao // resultado da última verificação
	empresaAtivaID    int                  // empresa em uso (protegida por mu)
}

const chaveEmpresaAtiva = "empresa_ativa"

// NewApp cria a instância da aplicação com um Store já inicializado.
// prepararBanco já deve ter rodado (garante ao menos uma empresa).
func NewApp(store storage.Store) *App {
	app := &App{store: store}
	app.empresaAtivaID = app.resolverEmpresaAtiva()

	if u, err := updater.New("informeai", "fynam", appVersion); err != nil {
		log.Printf("auto-update desativado: %v", err)
	} else {
		app.upd = u
	}
	return app
}

// resolverEmpresaAtiva devolve a empresa a usar: a salva em config, se ainda
// existir; senão a primeira da lista (persistindo a escolha). 0 se não houver
// nenhuma (não deve acontecer depois de prepararBanco).
func (a *App) resolverEmpresaAtiva() int {
	empresas, err := a.store.ListEmpresas(a.c())
	if err != nil || len(empresas) == 0 {
		return 0
	}

	if v, err := a.store.GetConfig(a.c(), chaveEmpresaAtiva); err == nil && v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			for _, e := range empresas {
				if e.ID == id {
					return id
				}
			}
		}
	}

	_ = a.store.SetConfig(a.c(), chaveEmpresaAtiva, strconv.Itoa(empresas[0].ID))
	return empresas[0].ID
}

// startup guarda o contexto do Wails e dispara a verificação de atualização.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if updaterHabilitado && a.upd != nil {
		go a.checarAtualizacaoNoInicio()
	}
}

// c devolve um contexto utilizável mesmo antes do startup.
func (a *App) c() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// empresa devolve o id da empresa ativa (todo dado é escopado por ela).
func (a *App) empresa() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.empresaAtivaID
}

// emitir dispara um evento para o frontend. Vira no-op quando não há
// runtime do Wails (testes, chamadas antes do startup).
func (a *App) emitir(evento string, dados ...interface{}) {
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, evento, dados...)
}

// =====================================================================
// Contas bancárias / caixas
// =====================================================================

func (a *App) ListContas() ([]model.Conta, error) {
	return a.store.ListContas(a.c(), a.empresa())
}

func (a *App) CreateConta(nome string, saldoInicial float64) (model.Conta, error) {
	return a.store.CreateConta(a.c(), a.empresa(), model.Conta{Nome: nome, SaldoInicial: saldoInicial})
}

func (a *App) UpdateConta(id int, nome string, saldoInicial float64) (model.Conta, error) {
	return a.store.UpdateConta(a.c(), model.Conta{ID: id, Nome: nome, SaldoInicial: saldoInicial})
}

func (a *App) DeleteConta(id int) error {
	return a.store.DeleteConta(a.c(), id)
}

// =====================================================================
// Categorias (plano de contas simplificado)
// =====================================================================

func (a *App) ListCategorias() ([]model.Categoria, error) {
	return a.store.ListCategorias(a.c(), a.empresa())
}

func (a *App) CreateCategoria(nome string, tipo string) (model.Categoria, error) {
	return a.store.CreateCategoria(a.c(), a.empresa(), model.Categoria{Nome: nome, Tipo: tipo})
}

func (a *App) DeleteCategoria(id int) error {
	return a.store.DeleteCategoria(a.c(), id)
}

// =====================================================================
// Lançamentos (contas a pagar / a receber)
// =====================================================================

// ListLancamentos delega os filtros estruturais ao Store e aplica aqui o
// filtro por Status (que é derivado das datas).
func (a *App) ListLancamentos(filtro model.LancamentoFiltro) ([]model.Lancamento, error) {
	itens, err := a.store.ListLancamentos(a.c(), a.empresa(), filtro)
	if err != nil {
		return nil, err
	}
	out := make([]model.Lancamento, 0, len(itens))
	for _, l := range itens {
		l = l.ComStatus()
		if filtro.Status != "" && l.Status != filtro.Status {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

func (a *App) CreateLancamento(in model.LancamentoInput) (model.Lancamento, error) {
	l := model.Lancamento{
		Tipo:           in.Tipo,
		Descricao:      in.Descricao,
		CategoriaID:    in.CategoriaID,
		ContaID:        in.ContaID,
		Valor:          in.Valor,
		DataVencimento: in.DataVencimento,
		DataPagamento:  "",
		Observacoes:    in.Observacoes,
	}
	criado, err := a.store.CreateLancamento(a.c(), a.empresa(), l)
	if err != nil {
		return model.Lancamento{}, err
	}
	return criado.ComStatus(), nil
}

func (a *App) UpdateLancamento(id int, in model.LancamentoInput) (model.Lancamento, error) {
	atual, err := a.store.GetLancamento(a.c(), id)
	if err != nil {
		return model.Lancamento{}, err
	}
	atual.Tipo = in.Tipo
	atual.Descricao = in.Descricao
	atual.CategoriaID = in.CategoriaID
	atual.ContaID = in.ContaID
	atual.Valor = in.Valor
	atual.DataVencimento = in.DataVencimento
	atual.Observacoes = in.Observacoes

	salvo, err := a.store.UpdateLancamento(a.c(), atual)
	if err != nil {
		return model.Lancamento{}, err
	}
	return salvo.ComStatus(), nil
}

func (a *App) DeleteLancamento(id int) error {
	return a.store.DeleteLancamento(a.c(), id)
}

// MarcarBaixa marca o lançamento como pago/recebido. dataPagamento vazia = hoje.
func (a *App) MarcarBaixa(id int, dataPagamento string) (model.Lancamento, error) {
	if dataPagamento == "" {
		dataPagamento = todayISO()
	}
	l, err := a.store.SetPagamento(a.c(), id, dataPagamento)
	if err != nil {
		return model.Lancamento{}, err
	}
	return l.ComStatus(), nil
}

// Estornar desfaz a baixa (volta o lançamento para em aberto).
func (a *App) Estornar(id int) (model.Lancamento, error) {
	l, err := a.store.SetPagamento(a.c(), id, "")
	if err != nil {
		return model.Lancamento{}, err
	}
	return l.ComStatus(), nil
}

// =====================================================================
// Dashboard
// =====================================================================

func (a *App) DashboardResumo() (Resumo, error) {
	emp := a.empresa()
	contas, err := a.store.ListContas(a.c(), emp)
	if err != nil {
		return Resumo{}, err
	}
	brutos, err := a.store.ListLancamentos(a.c(), emp, model.LancamentoFiltro{})
	if err != nil {
		return Resumo{}, err
	}
	lancs := make([]model.Lancamento, len(brutos))
	for i, l := range brutos {
		lancs[i] = l.ComStatus()
	}

	var saldoInicial float64
	for _, c := range contas {
		saldoInicial += c.SaldoInicial
	}

	var totalPago, totalRecebido, totalAPagar, totalAReceber float64
	for _, l := range lancs {
		switch {
		case l.Tipo == "pagar" && l.DataPagamento != "":
			totalPago += l.Valor
		case l.Tipo == "receber" && l.DataPagamento != "":
			totalRecebido += l.Valor
		}
		if l.Tipo == "pagar" && l.Status != "pago" {
			totalAPagar += l.Valor
		}
		if l.Tipo == "receber" && l.Status != "recebido" {
			totalAReceber += l.Valor
		}
	}
	saldoAtual := saldoInicial + totalRecebido - totalPago

	proximos := make([]model.Lancamento, 0)
	for _, l := range lancs {
		if l.Status != "pago" && l.Status != "recebido" {
			proximos = append(proximos, l)
		}
	}
	sort.SliceStable(proximos, func(i, j int) bool {
		return proximos[i].DataVencimento < proximos[j].DataVencimento
	})
	if len(proximos) > 8 {
		proximos = proximos[:8]
	}

	meses := ultimosMeses(6)
	fluxo := make([]FluxoMes, 0, len(meses))
	for _, m := range meses {
		var entradas, saidas float64
		for _, l := range lancs {
			if mesAno(l.DataVencimento) != m {
				continue
			}
			if l.Tipo == "receber" {
				entradas += l.Valor
			} else {
				saidas += l.Valor
			}
		}
		fluxo = append(fluxo, FluxoMes{Mes: m, Entradas: entradas, Saidas: saidas})
	}

	return Resumo{
		SaldoAtual:          saldoAtual,
		TotalAPagar:         totalAPagar,
		TotalAReceber:       totalAReceber,
		ProximosVencimentos: proximos,
		FluxoMensal:         fluxo,
		Hoje:                todayISO(),
	}, nil
}

// =====================================================================
// Relatórios
// =====================================================================

// RelatorioFluxoCaixa devolve a visão mensal (12 meses do ano informado)
// com entradas, saídas, saldo do mês e saldo acumulado.
func (a *App) RelatorioFluxoCaixa(ano int) ([]FluxoCaixaLinha, error) {
	emp := a.empresa()
	contas, err := a.store.ListContas(a.c(), emp)
	if err != nil {
		return nil, err
	}
	lancs, err := a.store.ListLancamentos(a.c(), emp, model.LancamentoFiltro{
		DataInicio: monthKey(ano, 1) + "-01",
		DataFim:    monthKey(ano, 12) + "-31",
	})
	if err != nil {
		return nil, err
	}

	var saldoAcumulado float64
	for _, c := range contas {
		saldoAcumulado += c.SaldoInicial
	}

	out := make([]FluxoCaixaLinha, 0, 12)
	for mes := 1; mes <= 12; mes++ {
		chave := monthKey(ano, mes)
		var entradas, saidas float64
		for _, l := range lancs {
			if mesAno(l.DataVencimento) != chave {
				continue
			}
			if l.Tipo == "receber" {
				entradas += l.Valor
			} else {
				saidas += l.Valor
			}
		}
		saldoPeriodo := entradas - saidas
		saldoAcumulado += saldoPeriodo
		out = append(out, FluxoCaixaLinha{
			Mes:            chave,
			Entradas:       entradas,
			Saidas:         saidas,
			SaldoPeriodo:   saldoPeriodo,
			SaldoAcumulado: saldoAcumulado,
		})
	}
	return out, nil
}

// RelatorioDRE devolve um DRE simplificado por período, agrupado por categoria.
func (a *App) RelatorioDRE(dataInicio string, dataFim string) (DRE, error) {
	emp := a.empresa()
	categorias, err := a.store.ListCategorias(a.c(), emp)
	if err != nil {
		return DRE{}, err
	}
	lancs, err := a.store.ListLancamentos(a.c(), emp, model.LancamentoFiltro{
		DataInicio: dataInicio,
		DataFim:    dataFim,
	})
	if err != nil {
		return DRE{}, err
	}

	totalPorCategoria := map[int]float64{}
	for _, l := range lancs {
		if l.CategoriaID == nil {
			continue
		}
		totalPorCategoria[*l.CategoriaID] += l.Valor
	}

	var linhas []DRELinha
	var receitaBruta, despesas float64
	for _, cat := range categorias {
		total := totalPorCategoria[cat.ID]
		if total <= 0 {
			continue
		}
		linhas = append(linhas, DRELinha{Categoria: cat.Nome, Tipo: cat.Tipo, Total: total})
		if cat.Tipo == "receita" {
			receitaBruta += total
		} else {
			despesas += total
		}
	}

	return DRE{
		Linhas:       linhas,
		ReceitaBruta: receitaBruta,
		Despesas:     despesas,
		Resultado:    receitaBruta - despesas,
	}, nil
}
