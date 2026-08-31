package main

import (
	"fmt"
	"os"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"fynam/internal/model"
	"fynam/internal/report"
)

// =====================================================================
// Exportação de relatórios (PDF / XLSX / CSV)
// =====================================================================
//
// Cada método público monta uma report.Tabela neutra e delega a
// serialização + diálogo "Salvar como" a exportar(). O frontend chama
// ExportarFluxoCaixa / ExportarDRE / ExportarLancamentos passando o
// formato ("pdf", "xlsx" ou "csv") e recebe de volta o caminho do arquivo
// gravado (ou "" se o usuário cancelar o diálogo).

// ExportarFluxoCaixa gera o relatório de fluxo de caixa do ano informado.
func (a *App) ExportarFluxoCaixa(ano int, formato string) (string, error) {
	linhas, err := a.RelatorioFluxoCaixa(ano)
	if err != nil {
		return "", err
	}
	return a.exportar(tabelaFluxoCaixa(ano, linhas), formato, fmt.Sprintf("fluxo-de-caixa-%d", ano))
}

// ExportarDRE gera o DRE simplificado do período informado.
func (a *App) ExportarDRE(dataInicio string, dataFim string, formato string) (string, error) {
	dre, err := a.RelatorioDRE(dataInicio, dataFim)
	if err != nil {
		return "", err
	}
	nome := fmt.Sprintf("dre_%s_a_%s", dataInicio, dataFim)
	return a.exportar(tabelaDRE(dataInicio, dataFim, dre), formato, nome)
}

// ExportarLancamentos gera a lista de lançamentos conforme os filtros.
func (a *App) ExportarLancamentos(filtro model.LancamentoFiltro, formato string) (string, error) {
	itens, err := a.ListLancamentos(filtro)
	if err != nil {
		return "", err
	}
	categorias, err := a.store.ListCategorias(a.c())
	if err != nil {
		return "", err
	}

	nome := "lancamentos"
	switch filtro.Tipo {
	case "pagar":
		nome = "contas-a-pagar"
	case "receber":
		nome = "contas-a-receber"
	}
	return a.exportar(tabelaLancamentos(filtro, itens, categorias), formato, nome)
}

// exportar serializa a tabela, pergunta onde salvar e grava o arquivo.
func (a *App) exportar(tab report.Tabela, formato string, nomeBase string) (string, error) {
	f := report.Formato(strings.ToLower(strings.TrimSpace(formato)))
	if !f.Valida() {
		return "", fmt.Errorf("formato inválido: %q (use pdf, xlsx ou csv)", formato)
	}

	dados, err := report.Gerar(tab, f)
	if err != nil {
		return "", err
	}

	caminho, err := wruntime.SaveFileDialog(a.c(), wruntime.SaveDialogOptions{
		Title:                "Salvar relatório",
		DefaultFilename:      nomeBase + "." + f.Extensao(),
		CanCreateDirectories: true,
		Filters: []wruntime.FileFilter{{
			DisplayName: strings.ToUpper(f.Extensao()) + " (*." + f.Extensao() + ")",
			Pattern:     "*." + f.Extensao(),
		}},
	})
	if err != nil {
		return "", err
	}
	if caminho == "" {
		return "", nil // usuário cancelou
	}

	if err := os.WriteFile(caminho, dados, 0o644); err != nil {
		return "", err
	}
	return caminho, nil
}

// ---------------------------------------------------------------------
// Construtores de tabela (puros — testáveis sem Wails)
// ---------------------------------------------------------------------

func tabelaFluxoCaixa(ano int, linhas []FluxoCaixaLinha) report.Tabela {
	t := report.Tabela{
		Titulo:    "Fluxo de Caixa",
		Subtitulo: fmt.Sprintf("Ano de %d", ano),
		Colunas: []report.Coluna{
			{Titulo: "Mês", Alinhar: report.Esquerda, Peso: 1.4},
			{Titulo: "Entradas", Alinhar: report.Direita, Peso: 1},
			{Titulo: "Saídas", Alinhar: report.Direita, Peso: 1},
			{Titulo: "Saldo do mês", Alinhar: report.Direita, Peso: 1.1},
			{Titulo: "Saldo acumulado", Alinhar: report.Direita, Peso: 1.3},
		},
	}

	var totEntradas, totSaidas, totPeriodo float64
	for _, l := range linhas {
		t.Linhas = append(t.Linhas, []report.Celula{
			report.Txt(rotuloMes(l.Mes)),
			report.Num(l.Entradas),
			report.Num(l.Saidas),
			report.Num(l.SaldoPeriodo),
			report.Num(l.SaldoAcumulado),
		})
		totEntradas += l.Entradas
		totSaidas += l.Saidas
		totPeriodo += l.SaldoPeriodo
	}

	var acumFinal float64
	if n := len(linhas); n > 0 {
		acumFinal = linhas[n-1].SaldoAcumulado
	}
	t.Rodape = [][]report.Celula{{
		report.Txt("Total"),
		report.Num(totEntradas),
		report.Num(totSaidas),
		report.Num(totPeriodo),
		report.Num(acumFinal),
	}}
	return t
}

func tabelaDRE(dataInicio string, dataFim string, dre DRE) report.Tabela {
	t := report.Tabela{
		Titulo:    "DRE simplificado",
		Subtitulo: fmt.Sprintf("Período de %s a %s", dataBR(dataInicio), dataBR(dataFim)),
		Colunas: []report.Coluna{
			{Titulo: "Categoria", Alinhar: report.Esquerda, Peso: 2.2},
			{Titulo: "Tipo", Alinhar: report.Esquerda, Peso: 1},
			{Titulo: "Total", Alinhar: report.Direita, Peso: 1.2},
		},
	}

	for _, l := range dre.Linhas {
		tipo := "Despesa"
		if l.Tipo == "receita" {
			tipo = "Receita"
		}
		t.Linhas = append(t.Linhas, []report.Celula{
			report.Txt(l.Categoria),
			report.Txt(tipo),
			report.Num(l.Total),
		})
	}

	t.Rodape = [][]report.Celula{
		{report.Txt("Receita Bruta"), report.Txt(""), report.Num(dre.ReceitaBruta)},
		{report.Txt("Despesas"), report.Txt(""), report.Num(dre.Despesas)},
		{report.Txt("Resultado do Período"), report.Txt(""), report.Num(dre.Resultado)},
	}
	return t
}

func tabelaLancamentos(filtro model.LancamentoFiltro, itens []model.Lancamento, categorias []model.Categoria) report.Tabela {
	nomeCategoria := make(map[int]string, len(categorias))
	for _, c := range categorias {
		nomeCategoria[c.ID] = c.Nome
	}

	titulo := "Lançamentos"
	switch filtro.Tipo {
	case "pagar":
		titulo = "Contas a Pagar"
	case "receber":
		titulo = "Contas a Receber"
	}

	var partesSub []string
	if filtro.Status != "" {
		partesSub = append(partesSub, "Status: "+rotuloStatus(filtro.Status))
	}
	if filtro.DataInicio != "" {
		partesSub = append(partesSub, "de "+dataBR(filtro.DataInicio))
	}
	if filtro.DataFim != "" {
		partesSub = append(partesSub, "até "+dataBR(filtro.DataFim))
	}
	if len(partesSub) == 0 {
		partesSub = append(partesSub, "Todos os registros")
	}

	t := report.Tabela{
		Titulo:    titulo,
		Subtitulo: strings.Join(partesSub, " · "),
		Colunas: []report.Coluna{
			{Titulo: "Descrição", Alinhar: report.Esquerda, Peso: 2.4},
			{Titulo: "Categoria", Alinhar: report.Esquerda, Peso: 1.6},
			{Titulo: "Vencimento", Alinhar: report.Centro, Peso: 1},
			{Titulo: "Pagamento", Alinhar: report.Centro, Peso: 1},
			{Titulo: "Valor", Alinhar: report.Direita, Peso: 1.1},
			{Titulo: "Status", Alinhar: report.Esquerda, Peso: 1},
		},
	}

	var total float64
	for _, l := range itens {
		categoria := "—"
		if l.CategoriaID != nil {
			if n, ok := nomeCategoria[*l.CategoriaID]; ok {
				categoria = n
			}
		}
		pagamento := "—"
		if l.DataPagamento != "" {
			pagamento = dataBR(l.DataPagamento)
		}
		t.Linhas = append(t.Linhas, []report.Celula{
			report.Txt(l.Descricao),
			report.Txt(categoria),
			report.Txt(dataBR(l.DataVencimento)),
			report.Txt(pagamento),
			report.Num(l.Valor),
			report.Txt(rotuloStatus(l.Status)),
		})
		total += l.Valor
	}

	t.Rodape = [][]report.Celula{{
		report.Txt("Total"),
		report.Txt(""),
		report.Txt(""),
		report.Txt(""),
		report.Num(total),
		report.Txt(""),
	}}
	return t
}
