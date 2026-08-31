package report

import (
	"bytes"
	"testing"
)

func tabelaExemplo() Tabela {
	return Tabela{
		Titulo:    "Fluxo de Caixa",
		Subtitulo: "Ano de 2026",
		Colunas: []Coluna{
			{Titulo: "Mês", Alinhar: Esquerda, Peso: 1.4},
			{Titulo: "Entradas", Alinhar: Direita, Peso: 1},
			{Titulo: "Saídas", Alinhar: Direita, Peso: 1},
		},
		Linhas: [][]Celula{
			{Txt("Ago/2026"), Num(3000), Num(1050.13)},
			{Txt("Set/2026"), Num(0), Num(815.01)},
		},
		Rodape: [][]Celula{
			{Txt("Total"), Num(3000), Num(1865.14)},
		},
	}
}

func TestFormatarMoeda(t *testing.T) {
	casos := map[float64]string{
		0:          "R$ 0,00",
		1050.13:    "R$ 1.050,13",
		-235.12:    "-R$ 235,12",
		1234567.89: "R$ 1.234.567,89",
		999.995:    "R$ 1.000,00", // arredondamento
	}
	for in, esperado := range casos {
		if got := FormatarMoeda(in); got != esperado {
			t.Errorf("FormatarMoeda(%v) = %q, esperado %q", in, got, esperado)
		}
	}
}

func TestGerarCSV(t *testing.T) {
	b, err := Gerar(tabelaExemplo(), FormatoCSV)
	if err != nil {
		t.Fatalf("CSV: %v", err)
	}
	if !bytes.HasPrefix(b, bomUTF8) {
		t.Error("CSV sem BOM UTF-8")
	}
	s := string(b)
	for _, sub := range []string{"Fluxo de Caixa", "Ago/2026", "R$ 1.050,13", "Total"} {
		if !bytes.Contains(b, []byte(sub)) {
			t.Errorf("CSV não contém %q\n%s", sub, s)
		}
	}
}

func TestGerarXLSX(t *testing.T) {
	b, err := Gerar(tabelaExemplo(), FormatoXLSX)
	if err != nil {
		t.Fatalf("XLSX: %v", err)
	}
	// arquivos .xlsx são ZIP: começam com "PK\x03\x04"
	if !bytes.HasPrefix(b, []byte("PK\x03\x04")) {
		t.Errorf("XLSX não parece um arquivo ZIP válido (%x)", b[:4])
	}
	if len(b) < 1000 {
		t.Errorf("XLSX suspeito de estar vazio: %d bytes", len(b))
	}
}

func TestGerarPDF(t *testing.T) {
	b, err := Gerar(tabelaExemplo(), FormatoPDF)
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Errorf("PDF não começa com %%PDF- (%q)", b[:5])
	}
	if !bytes.Contains(b, []byte("%%EOF")) {
		t.Error("PDF sem marcador EOF")
	}
}

func TestFormatoInvalido(t *testing.T) {
	if _, err := Gerar(tabelaExemplo(), Formato("txt")); err == nil {
		t.Error("esperava erro para formato inválido")
	}
}
