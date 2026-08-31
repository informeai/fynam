package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"fynam/internal/model"
	"fynam/internal/storage"
)

func novoStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// novaEmpresa cria uma empresa e devolve o id.
func novaEmpresa(t *testing.T, s *Store, nome string) int {
	t.Helper()
	e, err := s.CreateEmpresa(context.Background(), model.Empresa{Nome: nome, CriadaEm: "2026-01-01"})
	if err != nil {
		t.Fatalf("CreateEmpresa: %v", err)
	}
	return e.ID
}

func TestCRUDLancamento(t *testing.T) {
	ctx := context.Background()
	s := novoStore(t)
	emp := novaEmpresa(t, s, "Empresa A")

	conta, err := s.CreateConta(ctx, emp, model.Conta{Nome: "Caixa", SaldoInicial: 100})
	if err != nil || conta.ID == 0 {
		t.Fatalf("CreateConta: %v (id=%d)", err, conta.ID)
	}
	cat, err := s.CreateCategoria(ctx, emp, model.Categoria{Nome: "Vendas", Tipo: "receita"})
	if err != nil || cat.ID == 0 {
		t.Fatalf("CreateCategoria: %v", err)
	}

	l, err := s.CreateLancamento(ctx, emp, model.Lancamento{
		Tipo: "receber", Descricao: "Pedido 1",
		CategoriaID: &cat.ID, ContaID: &conta.ID,
		Valor: 250, DataVencimento: "2026-09-10",
	})
	if err != nil || l.ID == 0 {
		t.Fatalf("CreateLancamento: %v", err)
	}

	got, err := s.GetLancamento(ctx, l.ID)
	if err != nil {
		t.Fatalf("GetLancamento: %v", err)
	}
	if got.CategoriaID == nil || *got.CategoriaID != cat.ID {
		t.Fatalf("categoriaId não persistiu: %+v", got.CategoriaID)
	}

	// baixa e estorno
	if got, err = s.SetPagamento(ctx, l.ID, "2026-09-05"); err != nil || got.DataPagamento != "2026-09-05" {
		t.Fatalf("SetPagamento: %v data=%q", err, got.DataPagamento)
	}
	if got, err = s.SetPagamento(ctx, l.ID, ""); err != nil || got.DataPagamento != "" {
		t.Fatalf("estorno: %v data=%q", err, got.DataPagamento)
	}

	// filtro por tipo
	pag, err := s.ListLancamentos(ctx, emp, model.LancamentoFiltro{Tipo: "pagar"})
	if err != nil {
		t.Fatalf("ListLancamentos: %v", err)
	}
	if len(pag) != 0 {
		t.Fatalf("filtro tipo=pagar devia vir vazio, veio %d", len(pag))
	}

	// remover categoria => referência vira NULL, lançamento continua
	if err := s.DeleteCategoria(ctx, cat.ID); err != nil {
		t.Fatalf("DeleteCategoria: %v", err)
	}
	got, err = s.GetLancamento(ctx, l.ID)
	if err != nil {
		t.Fatalf("GetLancamento pós-delete: %v", err)
	}
	if got.CategoriaID != nil {
		t.Fatalf("categoria_id devia ser NULL após remover a categoria, veio %d", *got.CategoriaID)
	}

	if err := s.DeleteLancamento(ctx, l.ID); err != nil {
		t.Fatalf("DeleteLancamento: %v", err)
	}
	if _, err := s.GetLancamento(ctx, l.ID); !errors.Is(err, storage.ErrNaoEncontrado) {
		t.Fatalf("esperava ErrNaoEncontrado, veio %v", err)
	}
}

func TestErrosDeIDInexistente(t *testing.T) {
	ctx := context.Background()
	s := novoStore(t)

	if _, err := s.UpdateConta(ctx, model.Conta{ID: 999, Nome: "x"}); !errors.Is(err, storage.ErrNaoEncontrado) {
		t.Fatalf("UpdateConta id inexistente: %v", err)
	}
	if _, err := s.SetPagamento(ctx, 999, "2026-01-01"); !errors.Is(err, storage.ErrNaoEncontrado) {
		t.Fatalf("SetPagamento id inexistente: %v", err)
	}
}

func TestIsolamentoPorEmpresa(t *testing.T) {
	ctx := context.Background()
	s := novoStore(t)
	a := novaEmpresa(t, s, "A")
	b := novaEmpresa(t, s, "B")

	if _, err := s.CreateConta(ctx, a, model.Conta{Nome: "Caixa A"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateConta(ctx, b, model.Conta{Nome: "Caixa B1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateConta(ctx, b, model.Conta{Nome: "Caixa B2"}); err != nil {
		t.Fatal(err)
	}

	ca, _ := s.ListContas(ctx, a)
	cb, _ := s.ListContas(ctx, b)
	if len(ca) != 1 || len(cb) != 2 {
		t.Fatalf("isolamento falhou: A=%d B=%d", len(ca), len(cb))
	}

	// excluir a empresa B apaga suas contas em cascata
	if err := s.DeleteEmpresa(ctx, b); err != nil {
		t.Fatalf("DeleteEmpresa: %v", err)
	}
	cb, _ = s.ListContas(ctx, b)
	if len(cb) != 0 {
		t.Fatalf("cascade delete falhou: ainda há %d contas na empresa B", len(cb))
	}
	ca, _ = s.ListContas(ctx, a)
	if len(ca) != 1 {
		t.Fatalf("empresa A foi afetada indevidamente: %d contas", len(ca))
	}
}

func TestMigrarRegistrosSoltos(t *testing.T) {
	ctx := context.Background()
	s := novoStore(t)

	// simula dados de uma versão antiga: linhas sem empresa_id
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO contas (nome, saldo_inicial) VALUES ('Antiga', 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO categorias (nome, tipo) VALUES ('Antiga', 'receita')`); err != nil {
		t.Fatal(err)
	}

	emp := novaEmpresa(t, s, "Principal")
	n, err := s.MigrarRegistrosSoltos(ctx, emp)
	if err != nil {
		t.Fatalf("MigrarRegistrosSoltos: %v", err)
	}
	if n != 2 {
		t.Fatalf("esperava 2 registros movidos, veio %d", n)
	}
	if c, _ := s.ListContas(ctx, emp); len(c) != 1 {
		t.Fatalf("conta órfã não foi adotada: %d", len(c))
	}
}

// TestUpgradeDeBancoAntigo simula um fynam.db da versão sem múltiplas
// empresas: recria a tabela contas sem empresa_id, roda migrate de novo e
// confere que o ALTER adicionou a coluna e que o cascade de empresa funciona.
func TestUpgradeDeBancoAntigo(t *testing.T) {
	ctx := context.Background()
	s := novoStore(t)

	if _, err := s.db.ExecContext(ctx, `
		DROP TABLE lancamentos;
		DROP TABLE contas;
		CREATE TABLE contas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			nome TEXT NOT NULL,
			saldo_inicial REAL NOT NULL DEFAULT 0
		);
		CREATE TABLE lancamentos (
			id INTEGER PRIMARY KEY AUTOINCREMENT, tipo TEXT NOT NULL, descricao TEXT NOT NULL,
			categoria_id INTEGER, conta_id INTEGER, valor REAL NOT NULL DEFAULT 0,
			data_vencimento TEXT NOT NULL, data_pagamento TEXT NOT NULL DEFAULT '',
			observacoes TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO contas (nome) VALUES ('Legado');`); err != nil {
		t.Fatalf("preparar banco antigo: %v", err)
	}

	if err := s.migrate(ctx); err != nil {
		t.Fatalf("migrate (upgrade): %v", err)
	}

	emp := novaEmpresa(t, s, "Principal")
	if _, err := s.MigrarRegistrosSoltos(ctx, emp); err != nil {
		t.Fatalf("MigrarRegistrosSoltos: %v", err)
	}
	if c, _ := s.ListContas(ctx, emp); len(c) != 1 {
		t.Fatalf("conta legada não migrou: %d", len(c))
	}

	// cascade: apagar a empresa apaga a conta migrada (FK adicionada no ALTER)
	outra := novaEmpresa(t, s, "Outra")
	if err := s.DeleteEmpresa(ctx, emp); err != nil {
		t.Fatalf("DeleteEmpresa: %v", err)
	}
	_ = outra
	if c, _ := s.ListContas(ctx, emp); len(c) != 0 {
		t.Fatalf("cascade não funcionou após upgrade: %d contas restantes", len(c))
	}
}

func TestConfig(t *testing.T) {
	ctx := context.Background()
	s := novoStore(t)

	if v, err := s.GetConfig(ctx, "x"); err != nil || v != "" {
		t.Fatalf("GetConfig inexistente: %q %v", v, err)
	}
	if err := s.SetConfig(ctx, "empresa_ativa", "3"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetConfig(ctx, "empresa_ativa", "7"); err != nil { // upsert
		t.Fatal(err)
	}
	if v, _ := s.GetConfig(ctx, "empresa_ativa"); v != "7" {
		t.Fatalf("GetConfig = %q, esperado 7", v)
	}
}

// garante, em tempo de compilação, que *Store satisfaz a interface.
var _ storage.Store = (*Store)(nil)
