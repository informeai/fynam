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

// schemaTabelas cria as tabelas (em bancos novos, já com empresa_id) e os
// índices que não dependem de empresa_id.
const schemaTabelas = `
CREATE TABLE IF NOT EXISTS empresas (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    nome      TEXT NOT NULL,
    cnpj      TEXT NOT NULL DEFAULT '',
    criada_em TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS config (
    chave TEXT PRIMARY KEY,
    valor TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS contas (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    empresa_id    INTEGER REFERENCES empresas(id) ON DELETE CASCADE,
    nome          TEXT    NOT NULL,
    saldo_inicial REAL    NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS categorias (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    empresa_id INTEGER REFERENCES empresas(id) ON DELETE CASCADE,
    nome       TEXT    NOT NULL,
    tipo       TEXT    NOT NULL
);
CREATE TABLE IF NOT EXISTS lancamentos (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    empresa_id      INTEGER REFERENCES empresas(id)   ON DELETE CASCADE,
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

// schemaIndicesEmpresa só pode rodar depois que a coluna empresa_id existe
// em todas as tabelas (bancos antigos precisam do ALTER antes).
const schemaIndicesEmpresa = `
CREATE INDEX IF NOT EXISTS idx_contas_empresa      ON contas(empresa_id);
CREATE INDEX IF NOT EXISTS idx_categorias_empresa  ON categorias(empresa_id);
CREATE INDEX IF NOT EXISTS idx_lancamentos_empresa ON lancamentos(empresa_id);
`

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaTabelas); err != nil {
		return err
	}
	// Bancos criados antes do suporte a múltiplas empresas não têm a coluna
	// empresa_id — adiciona (as linhas existentes ficam com NULL e são
	// adotadas por MigrarRegistrosSoltos).
	for _, tab := range []string{"contas", "categorias", "lancamentos"} {
		if err := s.garantirColuna(ctx, tab, "empresa_id",
			"ALTER TABLE "+tab+" ADD COLUMN empresa_id INTEGER REFERENCES empresas(id) ON DELETE CASCADE"); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, schemaIndicesEmpresa); err != nil {
		return err
	}
	return nil
}

func (s *Store) garantirColuna(ctx context.Context, tabela, coluna, ddl string) error {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+tabela+")")
	if err != nil {
		return err
	}
	existe := false
	for rows.Next() {
		var (
			cid, notnull, pk int
			nome, tipo       string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &nome, &tipo, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		if nome == coluna {
			existe = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if existe {
		return nil
	}
	_, err = s.db.ExecContext(ctx, ddl)
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
// Empresas
// ---------------------------------------------------------------------

func (s *Store) ListEmpresas(ctx context.Context) ([]model.Empresa, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, nome, cnpj, criada_em FROM empresas ORDER BY nome, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.Empresa{}
	for rows.Next() {
		var e model.Empresa
		if err := rows.Scan(&e.ID, &e.Nome, &e.CNPJ, &e.CriadaEm); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) GetEmpresa(ctx context.Context, id int) (model.Empresa, error) {
	var e model.Empresa
	err := s.db.QueryRowContext(ctx,
		`SELECT id, nome, cnpj, criada_em FROM empresas WHERE id = ?`, id).
		Scan(&e.ID, &e.Nome, &e.CNPJ, &e.CriadaEm)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Empresa{}, storage.ErrNaoEncontrado
	}
	return e, err
}

func (s *Store) CreateEmpresa(ctx context.Context, e model.Empresa) (model.Empresa, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO empresas (nome, cnpj, criada_em) VALUES (?, ?, ?)`,
		e.Nome, e.CNPJ, e.CriadaEm)
	if err != nil {
		return model.Empresa{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Empresa{}, err
	}
	e.ID = int(id)
	return e, nil
}

func (s *Store) UpdateEmpresa(ctx context.Context, e model.Empresa) (model.Empresa, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE empresas SET nome = ?, cnpj = ? WHERE id = ?`, e.Nome, e.CNPJ, e.ID)
	if err != nil {
		return model.Empresa{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Empresa{}, storage.ErrNaoEncontrado
	}
	return s.GetEmpresa(ctx, e.ID)
}

func (s *Store) DeleteEmpresa(ctx context.Context, id int) error {
	// ON DELETE CASCADE cuida de contas, categorias e lançamentos.
	_, err := s.db.ExecContext(ctx, `DELETE FROM empresas WHERE id = ?`, id)
	return err
}

func (s *Store) MigrarRegistrosSoltos(ctx context.Context, empresaID int) (int, error) {
	var total int
	for _, tab := range []string{"contas", "categorias", "lancamentos"} {
		res, err := s.db.ExecContext(ctx,
			`UPDATE `+tab+` SET empresa_id = ? WHERE empresa_id IS NULL`, empresaID)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}
	return total, nil
}

// ---------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------

func (s *Store) GetConfig(ctx context.Context, chave string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT valor FROM config WHERE chave = ?`, chave).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) SetConfig(ctx context.Context, chave, valor string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO config (chave, valor) VALUES (?, ?)
		 ON CONFLICT(chave) DO UPDATE SET valor = excluded.valor`, chave, valor)
	return err
}

// ---------------------------------------------------------------------
// Contas
// ---------------------------------------------------------------------

func (s *Store) ListContas(ctx context.Context, empresaID int) ([]model.Conta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, nome, saldo_inicial FROM contas WHERE empresa_id = ? ORDER BY id`, empresaID)
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

func (s *Store) CreateConta(ctx context.Context, empresaID int, c model.Conta) (model.Conta, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO contas (empresa_id, nome, saldo_inicial) VALUES (?, ?, ?)`,
		empresaID, c.Nome, c.SaldoInicial)
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

func (s *Store) ListCategorias(ctx context.Context, empresaID int) ([]model.Categoria, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, nome, tipo FROM categorias WHERE empresa_id = ? ORDER BY id`, empresaID)
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

func (s *Store) CreateCategoria(ctx context.Context, empresaID int, c model.Categoria) (model.Categoria, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO categorias (empresa_id, nome, tipo) VALUES (?, ?, ?)`, empresaID, c.Nome, c.Tipo)
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

func (s *Store) ListLancamentos(ctx context.Context, empresaID int, f model.LancamentoFiltro) ([]model.Lancamento, error) {
	where := []string{"empresa_id = ?"}
	args := []any{empresaID}

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

	q := `SELECT ` + lancamentoCols + ` FROM lancamentos WHERE ` +
		strings.Join(where, " AND ") + ` ORDER BY data_vencimento, id`

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

func (s *Store) CreateLancamento(ctx context.Context, empresaID int, l model.Lancamento) (model.Lancamento, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO lancamentos
		 (empresa_id, tipo, descricao, categoria_id, conta_id, valor, data_vencimento, data_pagamento, observacoes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		empresaID, l.Tipo, l.Descricao, toNullInt(l.CategoriaID), toNullInt(l.ContaID),
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
