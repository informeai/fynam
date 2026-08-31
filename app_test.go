package main

import (
	"context"
	"path/filepath"
	"testing"

	"fynam/internal/model"
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

	if _, err := a.CreateConta("Caixa", 1000); err != nil {
		t.Fatal(err)
	}
	receita, _ := a.CreateCategoria("Vendas", "receita")
	despesa, _ := a.CreateCategoria("Aluguel", "despesa")

	// uma entrada já recebida e uma saída em aberto, no mesmo mês
	entrada, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "receber", Descricao: "Venda", CategoriaID: &receita.ID,
		Valor: 500, DataVencimento: "2026-05-10",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.MarcarBaixa(entrada.ID, "2026-05-10"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "pagar", Descricao: "Aluguel maio", CategoriaID: &despesa.ID,
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

	// vencido e não pago => status "atrasado"
	if _, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "pagar", Descricao: "Conta velha", Valor: 50, DataVencimento: "2000-01-01",
	}); err != nil {
		t.Fatal(err)
	}
	// futuro => "pendente"
	if _, err := a.CreateLancamento(model.LancamentoInput{
		Tipo: "pagar", Descricao: "Conta futura", Valor: 70, DataVencimento: "2099-01-01",
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

func TestMultiplasEmpresas(t *testing.T) {
	a := appDeTeste(t)

	// dado na Empresa Principal
	if _, err := a.CreateConta("Caixa Principal", 100); err != nil {
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
		Tipo: "pagar", Descricao: "Só da filial", Valor: 10, DataVencimento: "2026-06-01",
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
