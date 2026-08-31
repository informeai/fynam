package main

import (
	"context"
	"time"

	"fynam/internal/model"
	"fynam/internal/storage"
)

// prepararBanco garante que exista pelo menos uma empresa e que a primeira
// execução (ou o upgrade de uma versão sem múltiplas empresas) fique num
// estado consistente:
//
//   - se não há nenhuma empresa, cria "Empresa Principal";
//   - adota nela quaisquer contas/categorias/lançamentos "soltos" (dados de
//     versões anteriores);
//   - se essa empresa ficou sem cadastro nenhum, faz o seed padrão
//     (1 conta + plano de contas).
//
// É idempotente: em execuções seguintes (já com empresa) não faz nada.
func prepararBanco(ctx context.Context, store storage.Store) error {
	empresas, err := store.ListEmpresas(ctx)
	if err != nil {
		return err
	}
	if len(empresas) > 0 {
		return nil
	}

	emp, err := store.CreateEmpresa(ctx, model.Empresa{
		Nome:     "Empresa Principal",
		CriadaEm: time.Now().Format("2006-01-02"),
	})
	if err != nil {
		return err
	}

	if _, err := store.MigrarRegistrosSoltos(ctx, emp.ID); err != nil {
		return err
	}

	contas, err := store.ListContas(ctx, emp.ID)
	if err != nil {
		return err
	}
	cats, err := store.ListCategorias(ctx, emp.ID)
	if err != nil {
		return err
	}
	if len(contas) == 0 && len(cats) == 0 {
		return seedEmpresa(ctx, store, emp.ID)
	}
	return nil
}

// seedEmpresa popula uma empresa recém-criada com uma conta e o plano de
// contas básico, para não começar com todas as telas vazias.
func seedEmpresa(ctx context.Context, store storage.Store, empresaID int) error {
	if _, err := store.CreateConta(ctx, empresaID, model.Conta{
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
		if _, err := store.CreateCategoria(ctx, empresaID, c); err != nil {
			return err
		}
	}
	return nil
}
