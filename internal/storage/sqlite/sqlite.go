// Package sqlite implementa storage.Store sobre um arquivo SQLite local,
// usando o driver Go puro modernc.org/sqlite (sem CGO — compila para
// Windows/macOS/Linux sem toolchain C).
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"fynam/internal/model"
	"fynam/internal/storage"
)

// Store é a implementação SQLite de storage.Store.
type Store struct {
	db *sql.DB
}

// New abre (ou cria) o banco no caminho informado e garante o schema.
func New(path string) (*Store, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)",
		path,
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite serializa escritas; uma conexão evita "database is locked".
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS contas (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    nome          TEXT    NOT NULL,
    saldo_inicial REAL    NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS categorias (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    nome TEXT    NOT NULL,
    tipo TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS lancamentos (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    tipo            TEXT    NOT NULL,
    descricao       TEXT    NOT NULL,
    categoria_id    INTEGER REFERENCES categorias(id) ON DELETE SET NULL,
    conta_id        INTEGER REFERENCES contas(id)     ON DELETE SET NULL,
    valor           REAL    NOT NULL DEFAULT 0,
    data_vencimento TEXT    NOT NULL,
    data_pagamento  TEXT    NOT NULL DEFAULT '',
    observacoes     TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_lancamentos_venc ON lancamentos(data_vencimento);
CREATE INDEX IF NOT EXISTS idx_lancamentos_tipo ON lancamentos(tipo);
`

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// ---------------------------------------------------------------------
// Helpers de conversão *int <-> NULL
// ---------------------------------------------------------------------

func toNullInt(p *int) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*p), Valid: true}
}

func fromNullInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// ---------------------------------------------------------------------
// Contas
// ---------------------------------------------------------------------

func (s *Store) ListContas(ctx context.Context) ([]model.Conta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, nome, saldo_inicial FROM contas ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.Conta{}
	for rows.Next() {
		var c model.Conta
		if err := rows.Scan(&c.ID, &c.Nome, &c.SaldoInicial); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetConta(ctx context.Context, id int) (model.Conta, error) {
	var c model.Conta
	err := s.db.QueryRowContext(ctx,
		`SELECT id, nome, saldo_inicial FROM contas WHERE id = ?`, id).
		Scan(&c.ID, &c.Nome, &c.SaldoInicial)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Conta{}, storage.ErrNaoEncontrado
	}
	return c, err
}

func (s *Store) CreateConta(ctx context.Context, c model.Conta) (model.Conta, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO contas (nome, saldo_inicial) VALUES (?, ?)`, c.Nome, c.SaldoInicial)
	if err != nil {
		return model.Conta{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Conta{}, err
	}
	c.ID = int(id)
	return c, nil
}

func (s *Store) UpdateConta(ctx context.Context, c model.Conta) (model.Conta, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE contas SET nome = ?, saldo_inicial = ? WHERE id = ?`, c.Nome, c.SaldoInicial, c.ID)
	if err != nil {
		return model.Conta{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Conta{}, storage.ErrNaoEncontrado
	}
	return c, nil
}

func (s *Store) DeleteConta(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM contas WHERE id = ?`, id)
	return err
}

// ---------------------------------------------------------------------
// Categorias
// ---------------------------------------------------------------------

func (s *Store) ListCategorias(ctx context.Context) ([]model.Categoria, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, nome, tipo FROM categorias ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.Categoria{}
	for rows.Next() {
		var c model.Categoria
		if err := rows.Scan(&c.ID, &c.Nome, &c.Tipo); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCategoria(ctx context.Context, c model.Categoria) (model.Categoria, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO categorias (nome, tipo) VALUES (?, ?)`, c.Nome, c.Tipo)
	if err != nil {
		return model.Categoria{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Categoria{}, err
	}
	c.ID = int(id)
	return c, nil
}

func (s *Store) DeleteCategoria(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM categorias WHERE id = ?`, id)
	return err
}

// ---------------------------------------------------------------------
// Lançamentos
// ---------------------------------------------------------------------

const lancamentoCols = `id, tipo, descricao, categoria_id, conta_id, valor, data_vencimento, data_pagamento, observacoes`

func scanLancamento(sc interface{ Scan(...any) error }) (model.Lancamento, error) {
	var (
		l           model.Lancamento
		catID, ctID sql.NullInt64
	)
	err := sc.Scan(&l.ID, &l.Tipo, &l.Descricao, &catID, &ctID,
		&l.Valor, &l.DataVencimento, &l.DataPagamento, &l.Observacoes)
	if err != nil {
		return model.Lancamento{}, err
	}
	l.CategoriaID = fromNullInt(catID)
	l.ContaID = fromNullInt(ctID)
	return l, nil
}

func (s *Store) ListLancamentos(ctx context.Context, f model.LancamentoFiltro) ([]model.Lancamento, error) {
	var (
		where []string
		args  []any
	)
	if f.Tipo != "" {
		where = append(where, "tipo = ?")
		args = append(args, f.Tipo)
	}
	if f.DataInicio != "" {
		where = append(where, "data_vencimento >= ?")
		args = append(args, f.DataInicio)
	}
	if f.DataFim != "" {
		where = append(where, "data_vencimento <= ?")
		args = append(args, f.DataFim)
	}

	q := `SELECT ` + lancamentoCols + ` FROM lancamentos`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	q += ` ORDER BY data_vencimento, id`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.Lancamento{}
	for rows.Next() {
		l, err := scanLancamento(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) GetLancamento(ctx context.Context, id int) (model.Lancamento, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+lancamentoCols+` FROM lancamentos WHERE id = ?`, id)
	l, err := scanLancamento(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Lancamento{}, storage.ErrNaoEncontrado
	}
	return l, err
}

func (s *Store) CreateLancamento(ctx context.Context, l model.Lancamento) (model.Lancamento, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO lancamentos
		 (tipo, descricao, categoria_id, conta_id, valor, data_vencimento, data_pagamento, observacoes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.Tipo, l.Descricao, toNullInt(l.CategoriaID), toNullInt(l.ContaID),
		l.Valor, l.DataVencimento, l.DataPagamento, l.Observacoes)
	if err != nil {
		return model.Lancamento{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Lancamento{}, err
	}
	l.ID = int(id)
	return l, nil
}

func (s *Store) UpdateLancamento(ctx context.Context, l model.Lancamento) (model.Lancamento, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE lancamentos SET
		   tipo = ?, descricao = ?, categoria_id = ?, conta_id = ?,
		   valor = ?, data_vencimento = ?, observacoes = ?
		 WHERE id = ?`,
		l.Tipo, l.Descricao, toNullInt(l.CategoriaID), toNullInt(l.ContaID),
		l.Valor, l.DataVencimento, l.Observacoes, l.ID)
	if err != nil {
		return model.Lancamento{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Lancamento{}, storage.ErrNaoEncontrado
	}
	return s.GetLancamento(ctx, l.ID)
}

func (s *Store) DeleteLancamento(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM lancamentos WHERE id = ?`, id)
	return err
}

func (s *Store) SetPagamento(ctx context.Context, id int, dataPagamento string) (model.Lancamento, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE lancamentos SET data_pagamento = ? WHERE id = ?`, dataPagamento, id)
	if err != nil {
		return model.Lancamento{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Lancamento{}, storage.ErrNaoEncontrado
	}
	return s.GetLancamento(ctx, id)
}
