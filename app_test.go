package main

import (
	"context"
	"path/filepath"
	"testing"

	"fynam/internal/model"
	"fynam/internal/ofx"
	"fynam/internal/storage/sqlite"
)

// appDeTeste monta um App sobre um SQLite temporário, já com "Empresa
// Principal" criada e ativa (via prepararBanco), mas sem seed de contas.
func appDeTeste(t *testing.T) *App {
	t.Helper()
	st, err := sqlite.New(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := prepararBanco(context.Background(), st); err != nil {
		t.Fatalf("prepararBanco: %v", err)
	}
	// prepararBanco faz o seed padrão (conta + categorias); limpa para os
	// testes começarem do zero.
	limparEmpresaAtiva(t, st)
	return NewApp(st)
}

// contaDeTeste cria uma conta na empresa ativa e devolve seu id, para os
// testes que precisam lançar (ContaID é obrigatório).
func contaDeTeste(t *testing.T, a *App) int {
	t.Helper()
	c, err := a.CreateConta("Caixa", 0, "", "", "")
	if err != nil {
		t.Fatalf("CreateConta: %v", err)
	}
	return c.ID
}

func limparEmpresaAtiva(t *testing.T, st *sqlite.Store) {
	t.Helper()
	ctx := context.Background()
	emps, _ := st.ListEmpresas(ctx)
	if len(emps) == 0 {
		return
	}
	id := emps[0].ID
	contas, _ := st.ListContas(ctx, id)
	for _, c := range contas {
		_ = st.DeleteConta(ctx, c.ID)
	}
	cats, _ := st.ListCategorias(ctx, id)
	for _, c := range cats {
		_ = st.DeleteCategoria(ctx, c.ID)
	}
}

func TestDashboardEDRE(t *testing.T) {
	a := appDeTeste(t)
	_ = context.Background()

	caixa, err := a.CreateConta("Caixa", 1000, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	receita, _ := a.CreateCategoria("Vendas", "receita")
	despesa, _ := a.CreateCategoria("Aluguel", "despesa")

	// uma entrada já recebida e uma saída em aberto, no mesmo mês
	entrada, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "receber", Descricao: "Venda", CategoriaID: &receita.ID, ContaID: &caixa.ID,
		Valor: 500, DataVencimento: "2026-05-10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.MarcarBaixa(entrada.ID, "2026-05-10"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "pagar", Descricao: "Aluguel maio", CategoriaID: &despesa.ID, ContaID: &caixa.ID,
		Valor: 200, DataVencimento: "2026-05-20",
	}); err != nil {
		t.Fatal(err)
	}

	resumo, err := a.DashboardResumo()
	if err != nil {
		t.Fatalf("DashboardResumo: %v", err)
	}
	// saldo = 1000 inicial + 500 recebido - 0 pago
	if resumo.SaldoAtual != 1500 {
		t.Errorf("SaldoAtual = %v, esperado 1500", resumo.SaldoAtual)
	}
	if resumo.TotalAPagar != 200 {
		t.Errorf("TotalAPagar = %v, esperado 200", resumo.TotalAPagar)
	}
	if resumo.TotalAReceber != 0 {
		t.Errorf("TotalAReceber = %v, esperado 0", resumo.TotalAReceber)
	}

	dre, err := a.RelatorioDRE("2026-05-01", "2026-05-31")
	if err != nil {
		t.Fatalf("RelatorioDRE: %v", err)
	}
	if dre.ReceitaBruta != 500 || dre.Despesas != 200 || dre.Resultado != 300 {
		t.Errorf("DRE = %+v, esperado receita 500 / despesa 200 / resultado 300", dre)
	}

	fluxo, err := a.RelatorioFluxoCaixa(2026)
	if err != nil {
		t.Fatalf("RelatorioFluxoCaixa: %v", err)
	}
	if len(fluxo) != 12 {
		t.Fatalf("fluxo devia ter 12 meses, tem %d", len(fluxo))
	}
	maio := fluxo[4]
	if maio.Entradas != 500 || maio.Saidas != 200 || maio.SaldoPeriodo != 300 {
		t.Errorf("maio = %+v", maio)
	}
	// saldo acumulado de maio = 1000 + 300
	if maio.SaldoAcumulado != 1300 {
		t.Errorf("SaldoAcumulado maio = %v, esperado 1300", maio.SaldoAcumulado)
	}
}

func TestFiltroPorStatusDerivado(t *testing.T) {
	a := appDeTeste(t)
	conta := contaDeTeste(t, a)

	// vencido e não pago => status "atrasado"
	if _, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "pagar", Descricao: "Conta velha", ContaID: &conta, Valor: 50, DataVencimento: "2000-01-01",
	}); err != nil {
		t.Fatal(err)
	}
	// futuro => "pendente"
	if _, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "pagar", Descricao: "Conta futura", ContaID: &conta, Valor: 70, DataVencimento: "2099-01-01",
	}); err != nil {
		t.Fatal(err)
	}

	atrasados, err := a.ListLancamentos(model.LancamentoFiltro{Tipo: "pagar", Status: "atrasado"})
	if err != nil {
		t.Fatal(err)
	}
	if len(atrasados) != 1 || atrasados[0].Descricao != "Conta velha" {
		t.Fatalf("filtro status=atrasado: %+v", atrasados)
	}
}

func TestFiltrosPorCampos(t *testing.T) {
	a := appDeTeste(t)
	caixa := contaDeTeste(t, a)
	banco, err := a.CreateConta("Banco", 0, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	aluguel, _ := a.CreateCategoria("Aluguel", "despesa")
	energia, _ := a.CreateCategoria("Energia", "despesa")

	novo := func(desc string, catID *int, contaID int, valor float64, venc string) {
		t.Helper()
		if _, err := a.CreateLancamento(model.LancamentoInput{
			Tipo: "pagar", Descricao: desc, CategoriaID: catID, ContaID: &contaID,
			Valor: valor, DataVencimento: venc,
		}); err != nil {
			t.Fatalf("CreateLancamento %q: %v", desc, err)
		}
	}
	novo("Aluguel loja centro", &aluguel.ID, caixa, 2500, "2026-03-10")
	novo("Aluguel galpão", &aluguel.ID, banco.ID, 4000, "2026-03-15")
	novo("Conta de luz CEMIG", &energia.ID, caixa, 320, "2026-04-05")
	novo("Internet fibra", nil, caixa, 150, "2026-04-20")

	casos := []struct {
		nome   string
		filtro model.LancamentoFiltro
		quer   []string
	}{
		{"busca", model.LancamentoFiltro{Tipo: "pagar", Busca: "aluguel"},
			[]string{"Aluguel loja centro", "Aluguel galpão"}},
		{"busca case-insensitive", model.LancamentoFiltro{Tipo: "pagar", Busca: "CEMIG"},
			[]string{"Conta de luz CEMIG"}},
		{"categoria", model.LancamentoFiltro{Tipo: "pagar", CategoriaID: &energia.ID},
			[]string{"Conta de luz CEMIG"}},
		{"conta", model.LancamentoFiltro{Tipo: "pagar", ContaID: &banco.ID},
			[]string{"Aluguel galpão"}},
		{"valor min", model.LancamentoFiltro{Tipo: "pagar", ValorMin: f64(1000)},
			[]string{"Aluguel loja centro", "Aluguel galpão"}},
		{"faixa de valor", model.LancamentoFiltro{Tipo: "pagar", ValorMin: f64(100), ValorMax: f64(400)},
			[]string{"Conta de luz CEMIG", "Internet fibra"}},
		{"vencimento", model.LancamentoFiltro{Tipo: "pagar", DataInicio: "2026-04-01", DataFim: "2026-04-30"},
			[]string{"Conta de luz CEMIG", "Internet fibra"}},
		{"combinado", model.LancamentoFiltro{Tipo: "pagar", CategoriaID: &aluguel.ID, ContaID: &caixa, ValorMax: f64(3000)},
			[]string{"Aluguel loja centro"}},
	}

	for _, c := range casos {
		got, err := a.ListLancamentos(c.filtro)
		if err != nil {
			t.Fatalf("%s: %v", c.nome, err)
		}
		nomes := make([]string, len(got))
		for i, l := range got {
			nomes[i] = l.Descricao
		}
		if !mesmoConjunto(nomes, c.quer) {
			t.Errorf("%s: got %v, quer %v", c.nome, nomes, c.quer)
		}
	}
}

func f64(v float64) *float64 { return &v }

func mesmoConjunto(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		if m[s] == 0 {
			return false
		}
		m[s]--
	}
	return true
}

func TestConciliar(t *testing.T) {
	a := appDeTeste(t)
	conta := contaDeTeste(t, a)

	l, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "pagar", Descricao: "Fornecedor X", ContaID: &conta, Valor: 120, DataVencimento: "2026-06-01",
	})
	if err != nil {
		t.Fatal(err)
	}

	// ContaID é obrigatório
	if _, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "pagar", Descricao: "Sem conta", Valor: 10, DataVencimento: "2026-06-01",
	}); err == nil {
		t.Fatal("CreateLancamento devia recusar lançamento sem conta")
	}

	// não dá pra conciliar antes da baixa
	if _, err := a.Conciliar(l.ID, ""); err == nil {
		t.Fatal("Conciliar devia recusar lançamento ainda não pago")
	}

	if _, err := a.MarcarBaixa(l.ID, "2026-06-02"); err != nil {
		t.Fatal(err)
	}
	conc, err := a.Conciliar(l.ID, "2026-06-03")
	if err != nil {
		t.Fatalf("Conciliar: %v", err)
	}
	if conc.Status != "conciliado" || conc.DataConciliacao != "2026-06-03" {
		t.Fatalf("após conciliar: status=%q dataConciliacao=%q", conc.Status, conc.DataConciliacao)
	}

	// filtro por status derivado "conciliado"
	conciliados, err := a.ListLancamentos(model.LancamentoFiltro{Tipo: "pagar", Status: "conciliado"})
	if err != nil {
		t.Fatal(err)
	}
	if len(conciliados) != 1 || conciliados[0].ID != l.ID {
		t.Fatalf("filtro status=conciliado: %+v", conciliados)
	}

	// não conta como "a pagar" no dashboard
	resumo, err := a.DashboardResumo()
	if err != nil {
		t.Fatal(err)
	}
	if resumo.TotalAPagar != 0 {
		t.Errorf("TotalAPagar = %v, esperado 0 (lançamento conciliado)", resumo.TotalAPagar)
	}

	// desfazer conciliação volta para "pago"
	volta, err := a.DesfazerConciliacao(l.ID)
	if err != nil {
		t.Fatalf("DesfazerConciliacao: %v", err)
	}
	if volta.Status != "pago" || volta.DataConciliacao != "" {
		t.Fatalf("após desfazer: status=%q dataConciliacao=%q", volta.Status, volta.DataConciliacao)
	}

	// estornar a baixa de um lançamento conciliado limpa tudo
	if _, err := a.Conciliar(l.ID, ""); err != nil {
		t.Fatal(err)
	}
	est, err := a.Estornar(l.ID)
	if err != nil {
		t.Fatalf("Estornar: %v", err)
	}
	if est.DataPagamento != "" || est.DataConciliacao != "" {
		t.Fatalf("estorno devia limpar pagamento e conciliação: %+v", est)
	}
}

func TestDataPagamentoPeloFormulario(t *testing.T) {
	a := appDeTeste(t)
	conta := contaDeTeste(t, a)

	base := func(dataPagamento string) model.LancamentoInput {
		return model.LancamentoInput{
			Tipo: "receber", Descricao: "Adiantamento", ContaID: &conta,
			Valor: 200, DataVencimento: "2026-07-31", DataPagamento: dataPagamento,
		}
	}

	// criar já recebido, numa data diferente do vencimento
	l, err := a.CreateLancamento(base("2026-07-10"))
	if err != nil {
		t.Fatal(err)
	}
	if l.DataPagamento != "2026-07-10" || l.Status != "recebido" {
		t.Fatalf("criar com dataPagamento: %+v", l)
	}

	// editar mudando só a data de recebimento
	up, err := a.UpdateLancamento(l.ID, base("2026-07-12"))
	if err != nil {
		t.Fatal(err)
	}
	if up.DataPagamento != "2026-07-12" || up.Status != "recebido" {
		t.Fatalf("editar dataPagamento: %+v", up)
	}

	// limpar a data pelo formulário reabre o lançamento e desfaz a conciliação
	if _, err := a.Conciliar(l.ID, "2026-07-13"); err != nil {
		t.Fatal(err)
	}
	limpo, err := a.UpdateLancamento(l.ID, base(""))
	if err != nil {
		t.Fatal(err)
	}
	if limpo.DataPagamento != "" || limpo.DataConciliacao != "" || limpo.Status == "conciliado" {
		t.Fatalf("limpar dataPagamento devia reabrir e desconciliar: %+v", limpo)
	}
}

func TestImportacaoOFX(t *testing.T) {
	a := appDeTeste(t)
	conta := contaDeTeste(t, a)

	// um lançamento a pagar que deve casar com um débito do extrato
	aluguel, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "pagar", Descricao: "Aluguel", ContaID: &conta,
		Valor: 2400, DataVencimento: "2026-09-08",
	})
	if err != nil {
		t.Fatal(err)
	}

	ext := ofx.Extrato{
		Conta:      ofx.Conta{BankID: "341", AcctID: "111"},
		DataInicio: "2026-09-01", DataFim: "2026-09-30",
		Transacoes: []ofx.Transacao{
			{FITID: "A1", Data: "2026-09-09", Valor: 2400, Tipo: "pagar", Descricao: "ALUGUEL IMOB"},
			{FITID: "B2", Data: "2026-09-12", Valor: 1500, Tipo: "receber", Descricao: "CLIENTE XPTO"},
		},
	}

	daConta, err := a.lancamentosDaConta(conta)
	if err != nil {
		t.Fatal(err)
	}
	previa := montarPreviaOFX(ext, model.Conta{}, daConta, map[string]model.ExtratoRegistro{})

	if !previa.SemConflito || previa.Periodo == "" {
		t.Fatalf("prévia: semConflito=%v periodo=%q", previa.SemConflito, previa.Periodo)
	}
	if len(previa.Linhas) != 2 {
		t.Fatalf("prévia devia ter 2 linhas, tem %d", len(previa.Linhas))
	}
	l0 := previa.Linhas[0]
	if l0.Sugestao != "conciliar" || l0.SugestaoID == nil || *l0.SugestaoID != aluguel.ID {
		t.Fatalf("linha 0 devia sugerir conciliar com %d: %+v", aluguel.ID, l0)
	}
	if previa.Linhas[1].Sugestao != "criar" {
		t.Fatalf("linha 1 (sem candidato) devia sugerir criar: %+v", previa.Linhas[1])
	}

	// aplica: concilia a primeira, cria a segunda
	r, err := a.AplicarImportacaoOFX(conta, []DecisaoConciliacao{
		{Linha: previa.Linhas[0].Linha, Acao: "conciliar", LancamentoID: l0.SugestaoID},
		{Linha: previa.Linhas[1].Linha, Acao: "criar"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Conciliados != 1 || r.Criados != 1 || len(r.Erros) != 0 {
		t.Fatalf("resumo = %+v", r)
	}

	// o aluguel ficou conciliado com a data do extrato
	pagos, _ := a.ListLancamentos(model.LancamentoFiltro{Tipo: "pagar", Status: "conciliado"})
	if len(pagos) != 1 || pagos[0].ID != aluguel.ID ||
		pagos[0].DataPagamento != "2026-09-09" || pagos[0].DataConciliacao != "2026-09-09" {
		t.Fatalf("aluguel após conciliar: %+v", pagos)
	}

	// a segunda linha virou um lançamento a receber já conciliado
	recebidos, _ := a.ListLancamentos(model.LancamentoFiltro{Tipo: "receber", Status: "conciliado"})
	if len(recebidos) != 1 || recebidos[0].Valor != 1500 || recebidos[0].Descricao != "CLIENTE XPTO" {
		t.Fatalf("lançamento criado do extrato: %+v", recebidos)
	}

	// reimportar o mesmo extrato: ambas as linhas agora são "já importadas"
	daConta2, _ := a.lancamentosDaConta(conta)
	regs, err := a.store.ExtratoRegistrosPorFitid(a.c(), conta)
	if err != nil {
		t.Fatal(err)
	}
	previa2 := montarPreviaOFX(ext, model.Conta{}, daConta2, regs)
	for i, l := range previa2.Linhas {
		if !l.JaImportada || l.Sugestao != "ignorar" {
			t.Fatalf("reimport linha %d devia ser já importada/ignorar: %+v", i, l)
		}
	}

	// desconciliar o aluguel manualmente: a linha do extrato volta a ser
	// oferecida para reconciliação e a reimportação aplica de novo
	if _, err := a.DesfazerConciliacao(aluguel.ID); err != nil {
		t.Fatal(err)
	}
	daConta3, _ := a.lancamentosDaConta(conta)
	regs, _ = a.store.ExtratoRegistrosPorFitid(a.c(), conta)
	previa3 := montarPreviaOFX(ext, model.Conta{}, daConta3, regs)

	linhaAluguel := previa3.Linhas[0]
	if linhaAluguel.JaImportada || linhaAluguel.Sugestao != "conciliar" ||
		linhaAluguel.SugestaoID == nil || *linhaAluguel.SugestaoID != aluguel.ID {
		t.Fatalf("após desconciliar, a linha devia reabrir p/ conciliar com %d: %+v", aluguel.ID, linhaAluguel)
	}
	// a linha criada (B2) continua "já importada"
	if !previa3.Linhas[1].JaImportada {
		t.Fatalf("linha criada não deveria reabrir: %+v", previa3.Linhas[1])
	}

	r2, err := a.AplicarImportacaoOFX(conta, []DecisaoConciliacao{
		{Linha: linhaAluguel.Linha, Acao: "conciliar", LancamentoID: linhaAluguel.SugestaoID},
	})
	if err != nil || r2.Conciliados != 1 {
		t.Fatalf("reaplicar conciliação: r=%+v err=%v", r2, err)
	}
	pagos2, _ := a.ListLancamentos(model.LancamentoFiltro{Tipo: "pagar", Status: "conciliado"})
	if len(pagos2) != 1 || pagos2[0].ID != aluguel.ID {
		t.Fatalf("aluguel devia estar conciliado de novo: %+v", pagos2)
	}
}

func TestMultiplasEmpresas(t *testing.T) {
	a := appDeTeste(t)

	// dado na Empresa Principal
	if _, err := a.CreateConta("Caixa Principal", 100, "", "", ""); err != nil {
		t.Fatal(err)
	}

	// nova empresa passa a ser a ativa e vem com seed próprio
	emp2, err := a.CriarEmpresa("Filial Sul", "12.345.678/0001-99")
	if err != nil {
		t.Fatalf("CriarEmpresa: %v", err)
	}
	ativa, _ := a.EmpresaAtiva()
	if ativa.ID != emp2.ID {
		t.Fatalf("CriarEmpresa devia ativar a nova empresa (%d), ativa=%d", emp2.ID, ativa.ID)
	}
	cats, _ := a.ListCategorias()
	if len(cats) != 9 {
		t.Errorf("nova empresa devia ter 9 categorias de seed, tem %d", len(cats))
	}
	contas, _ := a.ListContas()
	if len(contas) != 1 || contas[0].Nome != "Caixa / Conta Principal" {
		t.Errorf("nova empresa: contas = %+v", contas)
	}

	// dado da Empresa Principal não vaza para a Filial
	if _, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "pagar", Descricao: "Só da filial", ContaID: &contas[0].ID,
		Valor: 10, DataVencimento: "2026-06-01",
	}); err != nil {
		t.Fatal(err)
	}

	empresas, _ := a.ListEmpresas()
	var principalID int
	for _, e := range empresas {
		if e.Nome == "Empresa Principal" {
			principalID = e.ID
		}
	}
	if err := a.TrocarEmpresa(principalID); err != nil {
		t.Fatalf("TrocarEmpresa: %v", err)
	}
	contas, _ = a.ListContas()
	if len(contas) != 1 || contas[0].Nome != "Caixa Principal" {
		t.Errorf("de volta na Principal: contas = %+v", contas)
	}
	lancs, _ := a.ListLancamentos(model.LancamentoFiltro{})
	if len(lancs) != 0 {
		t.Errorf("lançamento da filial vazou para a Principal: %+v", lancs)
	}

	// não pode excluir a última empresa
	if err := a.ExcluirEmpresa(emp2.ID); err != nil {
		t.Fatalf("ExcluirEmpresa: %v", err)
	}
	if err := a.ExcluirEmpresa(principalID); err == nil {
		t.Error("ExcluirEmpresa devia recusar apagar a única empresa")
	}
}
