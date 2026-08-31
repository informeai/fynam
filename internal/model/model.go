// Package model reúne os tipos de domínio do Fynam.
//
// Estes tipos são o contrato entre três camadas: o backend Go, qualquer
// implementação de armazenamento (ver internal/storage) e a interface web
// (as tags `json` definem exatamente como cada campo chega ao frontend).
package model

import "time"

// Empresa (ou filial) é a unidade de isolamento dos dados: cada conta,
// categoria e lançamento pertence a exatamente uma empresa. O app trabalha
// sempre com uma empresa ativa por vez.
type Empresa struct {
	ID       int    `json:"id"`
	Nome     string `json:"nome"`
	CNPJ     string `json:"cnpj"`
	CriadaEm string `json:"criadaEm"` // "YYYY-MM-DD"
}

// Conta bancária / caixa.
type Conta struct {
	ID           int     `json:"id"`
	Nome         string  `json:"nome"`
	SaldoInicial float64 `json:"saldoInicial"`
}

// Categoria do plano de contas simplificado. Tipo: "receita" | "despesa".
type Categoria struct {
	ID   int    `json:"id"`
	Nome string `json:"nome"`
	Tipo string `json:"tipo"`
}

// Lancamento é uma conta a pagar ou a receber.
// Tipo: "pagar" | "receber". DataPagamento vazia ("") significa em aberto.
// Status é derivado das datas em tempo de leitura e nunca é persistido.
type Lancamento struct {
	ID             int     `json:"id"`
	Tipo           string  `json:"tipo"`
	Descricao      string  `json:"descricao"`
	CategoriaID    *int    `json:"categoriaId"`
	ContaID        *int    `json:"contaId"`
	Valor          float64 `json:"valor"`
	DataVencimento string  `json:"dataVencimento"`
	DataPagamento  string  `json:"dataPagamento"`
	Observacoes    string  `json:"observacoes"`
	Status         string  `json:"status,omitempty"`
}

// LancamentoInput é o payload de criação/edição vindo do frontend.
type LancamentoInput struct {
	Tipo           string  `json:"tipo"`
	Descricao      string  `json:"descricao"`
	CategoriaID    *int    `json:"categoriaId"`
	ContaID        *int    `json:"contaId"`
	Valor          float64 `json:"valor"`
	DataVencimento string  `json:"dataVencimento"`
	Observacoes    string  `json:"observacoes"`
}

// LancamentoFiltro são filtros opcionais da listagem. Campo vazio = ignorado.
//
// Tipo, DataInicio e DataFim são resolvidos pela camada de armazenamento
// (viram WHERE no SQL, query nativa no Mongo, etc.). Status é derivado e,
// por isso, aplicado depois, na camada de aplicação.
type LancamentoFiltro struct {
	Tipo       string `json:"tipo"`
	Status     string `json:"status"`
	DataInicio string `json:"dataInicio"`
	DataFim    string `json:"dataFim"`
}

// DerivarStatus calcula o status a partir das datas, em vez de guardar um
// campo que poderia ficar desatualizado.
func (l Lancamento) DerivarStatus() string {
	if l.DataPagamento != "" {
		if l.Tipo == "pagar" {
			return "pago"
		}
		return "recebido"
	}
	if l.DataVencimento < time.Now().Format("2006-01-02") {
		return "atrasado"
	}
	return "pendente"
}

// ComStatus devolve uma cópia do lançamento com o campo Status preenchido.
func (l Lancamento) ComStatus() Lancamento {
	l.Status = l.DerivarStatus()
	return l
}
