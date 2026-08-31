// Package report gera relatórios tabulares do Fynam em PDF, XLSX e CSV.
//
// O fluxo é sempre o mesmo: a camada de aplicação monta uma Tabela neutra
// (títulos, colunas, linhas e rodapé) e chama Gerar com o formato desejado.
// Nenhum writer conhece as regras de negócio do Fynam — só recebem a Tabela.
package report

import (
	"fmt"
	"math"
	"strconv"
)

// Formato de saída suportado.
type Formato string

const (
	FormatoCSV  Formato = "csv"
	FormatoXLSX Formato = "xlsx"
	FormatoPDF  Formato = "pdf"
)

// Extensao devolve a extensão de arquivo (sem ponto).
func (f Formato) Extensao() string { return string(f) }

// Valida indica se o formato é reconhecido.
func (f Formato) Valida() bool {
	switch f {
	case FormatoCSV, FormatoXLSX, FormatoPDF:
		return true
	}
	return false
}

// Alinhamento do conteúdo de uma coluna.
type Alinhamento int

const (
	Esquerda Alinhamento = iota
	Direita
	Centro
)

// Coluna descreve um cabeçalho e como alinhar a coluna.
type Coluna struct {
	Titulo  string
	Alinhar Alinhamento
	// Peso é a largura relativa da coluna no PDF (0 = 1).
	Peso float64
}

// Celula é o conteúdo de uma posição da tabela. Quando Valor != nil, a
// célula representa um número (moeda) e é escrita tipada no XLSX.
type Celula struct {
	Texto string
	Valor *float64
}

// Txt cria uma célula de texto puro.
func Txt(s string) Celula { return Celula{Texto: s} }

// Num cria uma célula monetária: texto formatado em pt-BR + valor tipado.
func Num(v float64) Celula {
	vv := v
	return Celula{Texto: FormatarMoeda(v), Valor: &vv}
}

// Tabela é a representação neutra de um relatório.
type Tabela struct {
	Titulo    string
	Subtitulo string
	Colunas   []Coluna
	Linhas    [][]Celula
	// Rodape são linhas destacadas (totais) impressas após as linhas normais.
	Rodape [][]Celula
}

// Gerar renderiza a tabela no formato pedido e devolve os bytes do arquivo.
func Gerar(t Tabela, f Formato) ([]byte, error) {
	switch f {
	case FormatoCSV:
		return gerarCSV(t)
	case FormatoXLSX:
		return gerarXLSX(t)
	case FormatoPDF:
		return gerarPDF(t)
	default:
		return nil, fmt.Errorf("formato de relatório não suportado: %q", f)
	}
}

// FormatarMoeda formata um valor como "R$ 1.234,56" (padrão pt-BR),
// com sinal negativo à frente quando for o caso.
func FormatarMoeda(v float64) string {
	neg := math.Signbit(v) && v != 0
	if neg {
		v = -v
	}
	cents := int64(math.Round(v * 100))
	reais := cents / 100
	frac := cents % 100

	s := strconv.FormatInt(reais, 10)
	var b []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b = append(b, '.')
		}
		b = append(b, s[i])
	}
	out := fmt.Sprintf("R$ %s,%02d", string(b), frac)
	if neg {
		out = "-" + out
	}
	return out
}

// textos devolve o Texto de cada célula (usado por CSV e PDF).
func textos(cells []Celula) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = c.Texto
	}
	return out
}
