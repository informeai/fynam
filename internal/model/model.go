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
//
// BankID, AcctID e AcctType são os identificadores usados na conciliação
// por importação de extrato (OFX/CSV): permitem casar o cabeçalho
// <BANKACCTFROM> do arquivo com a conta certa e recusar um arquivo do
// banco errado. São opcionais — uma conta-caixa interna pode deixá-los
// em branco. AcctType segue os valores do OFX ("CHECKING", "SAVINGS"...).
type Conta struct {
	ID           int     `json:"id"`
	Nome         string  `json:"nome"`
	SaldoInicial float64 `json:"saldoInicial"`
	BankID       string  `json:"bankId"`
	AcctID       string  `json:"acctId"`
	AcctType     string  `json:"acctType"`
}

// Categoria do plano de contas simplificado. Tipo: "receita" | "despesa".
type Categoria struct {
	ID   int    `json:"id"`
	Nome string `json:"nome"`
	Tipo string `json:"tipo"`
}

// Lancamento é uma conta a pagar ou a receber.
// Tipo: "pagar" | "receber". DataPagamento vazia ("") significa em aberto.
//
// Status é derivado em tempo de leitura e nunca é persistido. A derivação
// usa as datas e mais um único fato guardado além delas: DataConciliacao,
// preenchida quando o operador valida a conciliação do lançamento (só é
// possível depois da baixa). DataConciliacao vazia ("") = não conciliado.
type Lancamento struct {
	ID              int     `json:"id"`
	Tipo            string  `json:"tipo"`
	Descricao       string  `json:"descricao"`
	CategoriaID     *int    `json:"categoriaId"`
	ContaID         *int    `json:"contaId"`
	Valor           float64 `json:"valor"`
	DataVencimento  string  `json:"dataVencimento"`
	DataPagamento   string  `json:"dataPagamento"`
	DataConciliacao string  `json:"dataConciliacao"`
	Observacoes     string  `json:"observacoes"`
	Status          string  `json:"status,omitempty"`
}

// LancamentoInput é o payload de criação/edição vindo do frontend.
// ContaID é obrigatório (todo lançamento pertence a uma conta/caixa);
// CategoriaID continua opcional.
//
// DataPagamento é a data real da liquidação (pagamento, se "pagar";
// recebimento, se "receber") e é independente do vencimento. Vazia ("")
// = ainda em aberto. Limpá-la num lançamento conciliado também desfaz a
// conciliação.
type LancamentoInput struct {
	Tipo           string  `json:"tipo"`
	Descricao      string  `json:"descricao"`
	CategoriaID    *int    `json:"categoriaId"`
	ContaID        *int    `json:"contaId"`
	Valor          float64 `json:"valor"`
	DataVencimento string  `json:"dataVencimento"`
	DataPagamento  string  `json:"dataPagamento"`
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

// Liquidado indica que o lançamento já saiu do "em aberto" (foi baixado).
// Vale tanto para "pago"/"recebido" quanto para "conciliado".
func (l Lancamento) Liquidado() bool {
	return l.DataPagamento != ""
}

// ExtratoLinha é uma transação lida de um arquivo OFX, já normalizada.
// É persistida (por conta + FITID) para deduplicar importações futuras.
type ExtratoLinha struct {
	FITID     string  `json:"fitid"`
	Data      string  `json:"data"`  // "YYYY-MM-DD" (data de compensação no banco)
	Valor     float64 `json:"valor"` // sempre positivo
	Tipo      string  `json:"tipo"`  // "pagar" (débito) | "receber" (crédito)
	Descricao string  `json:"descricao"`
}

// ExtratoRegistro é o que se sabe sobre um FITID já importado numa conta:
// o que foi feito com ele e a qual lançamento ficou vinculado (nulo se
// ignorado, ou se o lançamento vinculado foi depois excluído).
type ExtratoRegistro struct {
	Status       string // "conciliado" | "criado" | "ignorado"
	LancamentoID *int
}

// DerivarStatus calcula o status a partir das datas (e de DataConciliacao),
// em vez de guardar um campo que poderia ficar desatualizado.
//
// A ordem importa: "conciliado" é o estado final e fica acima de
// "pago"/"recebido"; estes, por sua vez, ficam acima de "atrasado"/"pendente".
func (l Lancamento) DerivarStatus() string {
	if l.DataConciliacao != "" {
		return "conciliado"
	}
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
