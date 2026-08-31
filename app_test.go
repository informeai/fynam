package main

import (
	"context"
	"path/filepath"
	"testing"

	"fynam/internal/model"
	"fynam/internal/storage/sqlite"
)

// appDeTeste monta um App sobre um SQLite temporário, sem seed.
func appDeTeste(t *testing.T) *App {
	t.Helper()
	st, err := sqlite.New(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return NewApp(st)
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
