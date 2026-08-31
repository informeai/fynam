package main

import (
	"context"
	"sort"

	"fynam/internal/model"
	"fynam/internal/storage"
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
}

// NewApp cria a instância da aplicação com um Store já inicializado.
func NewApp(store storage.Store) *App {
	return &App{store: store}
}

// startup guarda o contexto do Wails (necessário para chamadas de runtime).
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// c devolve um contexto utilizável mesmo antes do startup.
func (a *App) c() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// =====================================================================
// Contas bancárias / caixas
// =====================================================================

func (a *App) ListContas() ([]model.Conta, error) {
	return a.store.ListContas(a.c())
}

func (a *App) CreateConta(nome string, saldoInicial float64) (model.Conta, error) {
	return a.store.CreateConta(a.c(), model.Conta{Nome: nome, SaldoInicial: saldoInicial})
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
	return a.store.ListCategorias(a.c())
}

func (a *App) CreateCategoria(nome string, tipo string) (model.Categoria, error) {
	return a.store.CreateCategoria(a.c(), model.Categoria{Nome: nome, Tipo: tipo})
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
	itens, err := a.store.ListLancamentos(a.c(), filtro)
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
	criado, err := a.store.CreateLancamento(a.c(), l)
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
	contas, err := a.store.ListContas(a.c())
	if err != nil {
		return Resumo{}, err
	}
	brutos, err := a.store.ListLancamentos(a.c(), model.LancamentoFiltro{})
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
	contas, err := a.store.ListContas(a.c())
	if err != nil {
		return nil, err
	}
	lancs, err := a.store.ListLancamentos(a.c(), model.LancamentoFiltro{
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
	categorias, err := a.store.ListCategorias(a.c())
	if err != nil {
		return DRE{}, err
	}
	lancs, err := a.store.ListLancamentos(a.c(), model.LancamentoFiltro{
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
