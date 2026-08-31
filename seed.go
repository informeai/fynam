package main

import (
	"context"

	"fynam/internal/model"
	"fynam/internal/storage"
)

// seedIfEmpty cria um cadastro básico (1 conta + plano de contas) na
// primeira execução, de forma independente do backend de persistência.
// Se o repositório já tiver contas, não faz nada.
func seedIfEmpty(ctx context.Context, store storage.Store) error {
	contas, err := store.ListContas(ctx)
	if err != nil {
		return err
	}
	if len(contas) > 0 {
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
