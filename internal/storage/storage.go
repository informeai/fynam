// Package storage define a interface de persistência do Fynam,
// independente de tecnologia.
//
// A aplicação (pacote main) só conhece a interface Store. Hoje existe uma
// implementação em SQLite (internal/storage/sqlite); amanhã pode existir
// uma para Postgres, MongoDB, Firebase, Oracle, etc. — basta satisfazer a
// mesma interface e trocar a linha de construção em main.go.
//
// Regras do contrato:
//   - Métodos Create* recebem a entidade SEM id (id == 0) e devolvem a
//     entidade já com o id atribuído pelo repositório.
//   - Operação sobre id inexistente devolve ErrNaoEncontrado.
//   - Nenhuma implementação deve persistir Lancamento.Status (é derivado).
//   - ListLancamentos aplica apenas os filtros "estruturais" (Tipo,
//     DataInicio, DataFim). O filtro por Status é responsabilidade da
//     camada de aplicação, pois Status é calculado.
//   - Ao remover uma conta/categoria, os lançamentos que a referenciavam
//     devem ficar com a referência nula (não apagar o lançamento).
package storage

import (
	"context"
	"errors"

	"fynam/internal/model"
)

// ErrNaoEncontrado é devolvido quando um id informado não existe.
var ErrNaoEncontrado = errors.New("registro não encontrado")

// Store é o contrato de persistência. Toda implementação precisa ser
// segura para uso concorrente (o Wails chama métodos em goroutines).
type Store interface {
	// Close libera os recursos do repositório (conexões, arquivos...).
	Close() error

	// ----- Contas -----
	ListContas(ctx context.Context) ([]model.Conta, error)
	GetConta(ctx context.Context, id int) (model.Conta, error)
	CreateConta(ctx context.Context, c model.Conta) (model.Conta, error)
	UpdateConta(ctx context.Context, c model.Conta) (model.Conta, error)
	DeleteConta(ctx context.Context, id int) error

	// ----- Categorias -----
	ListCategorias(ctx context.Context) ([]model.Categoria, error)
	CreateCategoria(ctx context.Context, c model.Categoria) (model.Categoria, error)
	DeleteCategoria(ctx context.Context, id int) error

	// ----- Lançamentos -----
	ListLancamentos(ctx context.Context, f model.LancamentoFiltro) ([]model.Lancamento, error)
	GetLancamento(ctx context.Context, id int) (model.Lancamento, error)
	CreateLancamento(ctx context.Context, l model.Lancamento) (model.Lancamento, error)
	UpdateLancamento(ctx context.Context, l model.Lancamento) (model.Lancamento, error)
	DeleteLancamento(ctx context.Context, id int) error

	// SetPagamento marca (data != "") ou desfaz (data == "") a baixa de um
	// lançamento, de forma atômica.
	SetPagamento(ctx context.Context, id int, dataPagamento string) (model.Lancamento, error)
}
