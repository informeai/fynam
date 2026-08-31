package main

import (
	"context"
	"encoding/json"
	"os"

	"fynam/internal/model"
	"fynam/internal/storage"
)

// seedIfEmpty prepara os dados na primeira execução, de forma independente
// do backend de persistência:
//
//  1. se o repositório já tem contas, não faz nada;
//  2. se existir o arquivo legado xfin-data.json (versão Electron/lowdb),
//     importa contas, categorias e lançamentos dele;
//  3. senão, cria um cadastro básico (1 conta + plano de contas).
func seedIfEmpty(ctx context.Context, store storage.Store, legacyJSONPath string) error {
	contas, err := store.ListContas(ctx)
	if err != nil {
		return err
	}
	if len(contas) > 0 {
		return nil
	}

	importado, err := importarLegado(ctx, store, legacyJSONPath)
	if err != nil {
		return err
	}
	if importado {
		return nil
	}
	return seedPadrao(ctx, store)
}

func seedPadrao(ctx context.Context, store storage.Store) error {
	if _, err := store.CreateConta(ctx, model.Conta{
		Nome: "Caixa / Conta Principal", SaldoInicial: 0,
	}); err != nil {
		return err
	}

	padrao := []model.Categoria{
		{Nome: "Vendas", Tipo: "receita"},
		{Nome: "Serviços prestados", Tipo: "receita"},
		{Nome: "Outras receitas", Tipo: "receita"},
		{Nome: "Fornecedores", Tipo: "despesa"},
		{Nome: "Folha de pagamento", Tipo: "despesa"},
		{Nome: "Aluguel", Tipo: "despesa"},
		{Nome: "Impostos", Tipo: "despesa"},
		{Nome: "Marketing", Tipo: "despesa"},
		{Nome: "Outras despesas", Tipo: "despesa"},
	}
	for _, c := range padrao {
		if _, err := store.CreateCategoria(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// formato do arquivo legado gerado pela versão lowdb.
type legadoDB struct {
	Contas []struct {
		ID           int     `json:"id"`
		Nome         string  `json:"nome"`
		SaldoInicial float64 `json:"saldoInicial"`
	} `json:"contas"`
	Categorias []struct {
		ID   int    `json:"id"`
		Nome string `json:"nome"`
		Tipo string `json:"tipo"`
	} `json:"categorias"`
	Lancamentos []struct {
		Tipo           string  `json:"tipo"`
		Descricao      string  `json:"descricao"`
		CategoriaID    *int    `json:"categoriaId"`
		ContaID        *int    `json:"contaId"`
		Valor          float64 `json:"valor"`
		DataVencimento string  `json:"dataVencimento"`
		DataPagamento  *string `json:"dataPagamento"`
		Observacoes    string  `json:"observacoes"`
	} `json:"lancamentos"`
}

// importarLegado devolve (true, nil) se importou algo do arquivo JSON antigo.
// Arquivo ausente => (false, nil), sem erro.
func importarLegado(ctx context.Context, store storage.Store, path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var antigo legadoDB
	if err := json.Unmarshal(raw, &antigo); err != nil {
		return false, err
	}
	if len(antigo.Contas) == 0 && len(antigo.Categorias) == 0 && len(antigo.Lancamentos) == 0 {
		return false, nil
	}

	// Os ids são reatribuídos pelo repositório; guardamos o mapa antigo->novo
	// para remapear as referências dos lançamentos.
	mapaConta := map[int]int{}
	for _, c := range antigo.Contas {
		nova, err := store.CreateConta(ctx, model.Conta{Nome: c.Nome, SaldoInicial: c.SaldoInicial})
		if err != nil {
			return false, err
		}
		mapaConta[c.ID] = nova.ID
	}

	mapaCategoria := map[int]int{}
	for _, c := range antigo.Categorias {
		nova, err := store.CreateCategoria(ctx, model.Categoria{Nome: c.Nome, Tipo: c.Tipo})
		if err != nil {
			return false, err
		}
		mapaCategoria[c.ID] = nova.ID
	}

	remap := func(id *int, m map[int]int) *int {
		if id == nil {
			return nil
		}
		if novo, ok := m[*id]; ok {
			return &novo
		}
		return nil
	}

	for _, l := range antigo.Lancamentos {
		dataPagamento := ""
		if l.DataPagamento != nil {
			dataPagamento = *l.DataPagamento
		}
		novo := model.Lancamento{
			Tipo:           l.Tipo,
			Descricao:      l.Descricao,
			CategoriaID:    remap(l.CategoriaID, mapaCategoria),
			ContaID:        remap(l.ContaID, mapaConta),
			Valor:          l.Valor,
			DataVencimento: l.DataVencimento,
			DataPagamento:  dataPagamento,
			Observacoes:    l.Observacoes,
		}
		if _, err := store.CreateLancamento(ctx, novo); err != nil {
			return false, err
		}
	}
	return true, nil
}
