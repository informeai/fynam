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

func TestCRUDLancamento(t *testing.T) {
	ctx := context.Background()
	s := novoStore(t)

	conta, err := s.CreateConta(ctx, model.Conta{Nome: "Caixa", SaldoInicial: 100})
	if err != nil || conta.ID == 0 {
		t.Fatalf("CreateConta: %v (id=%d)", err, conta.ID)
	}
	cat, err := s.CreateCategoria(ctx, model.Categoria{Nome: "Vendas", Tipo: "receita"})
	if err != nil || cat.ID == 0 {
		t.Fatalf("CreateCategoria: %v", err)
	}

	l, err := s.CreateLancamento(ctx, model.Lancamento{
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
	pag, err := s.ListLancamentos(ctx, model.LancamentoFiltro{Tipo: "pagar"})
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

// garante, em tempo de compilação, que *Store satisfaz a interface.
var _ storage.Store = (*Store)(nil)
