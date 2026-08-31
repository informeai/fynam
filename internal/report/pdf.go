package report

import (
	"bytes"

	"github.com/go-pdf/fpdf"
)

// gerarPDF produz um PDF A4 retrato com título, subtítulo, tabela com
// cabeçalho repetido a cada página e linhas zebradas, e rodapé em negrito.
func gerarPDF(t Tabela) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("") // cp1252: cobre acentos pt-BR
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(false, 15) // controlamos a quebra manualmente, por linha

	pageW, pageH := pdf.GetPageSize()
	usableW := pageW - 30
	const limiteInferior = 15.0

	// larguras absolutas das colunas a partir dos pesos relativos
	var somaPeso float64
	for _, c := range t.Colunas {
		p := c.Peso
		if p == 0 {
			p = 1
		}
		somaPeso += p
	}
	larg := make([]float64, len(t.Colunas))
	for i, c := range t.Colunas {
		p := c.Peso
		if p == 0 {
			p = 1
		}
		larg[i] = usableW * p / somaPeso
	}

	alinhar := func(i int) string {
		if i >= len(t.Colunas) {
			return "L"
		}
		switch t.Colunas[i].Alinhar {
		case Direita:
			return "R"
		case Centro:
			return "C"
		default:
			return "L"
		}
	}

	desenharCabecalho := func() {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetFillColor(226, 232, 240)
		pdf.SetTextColor(15, 23, 42)
		for i, c := range t.Colunas {
			pdf.CellFormat(larg[i], 7, tr(c.Titulo), "1", 0, alinhar(i), true, 0, "")
		}
		pdf.Ln(-1)
	}

	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() == 1 {
			return
		}
		pdf.SetY(12)
		desenharCabecalho()
	})

	pdf.AddPage()

	if t.Titulo != "" {
		pdf.SetFont("Helvetica", "B", 16)
		pdf.SetTextColor(15, 23, 42)
		pdf.CellFormat(usableW, 9, tr(t.Titulo), "", 1, "L", false, 0, "")
	}
	if t.Subtitulo != "" {
		pdf.SetFont("Helvetica", "", 10)
		pdf.SetTextColor(100, 116, 139)
		pdf.CellFormat(usableW, 6, tr(t.Subtitulo), "", 1, "L", false, 0, "")
	}
	pdf.Ln(3)
	desenharCabecalho()

	const alturaLinha = 6.5
	fill := false
	for _, ln := range t.Linhas {
		if pdf.GetY()+alturaLinha > pageH-limiteInferior {
			pdf.AddPage() // dispara o HeaderFunc, que redesenha o cabeçalho
			fill = false
		}
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetFillColor(248, 250, 252)
		pdf.SetTextColor(15, 23, 42)
		for i := range t.Colunas {
			txt := ""
			if i < len(ln) {
				txt = ln[i].Texto
			}
			pdf.CellFormat(larg[i], alturaLinha, tr(txt), "1", 0, alinhar(i), fill, 0, "")
		}
		pdf.Ln(-1)
		fill = !fill
	}

	for _, r := range t.Rodape {
		if pdf.GetY()+7 > pageH-limiteInferior {
			pdf.AddPage()
		}
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetFillColor(237, 242, 247)
		pdf.SetTextColor(15, 23, 42)
		for i := range t.Colunas {
			txt := ""
			if i < len(r) {
				txt = r[i].Texto
			}
			pdf.CellFormat(larg[i], 7, tr(txt), "1", 0, alinhar(i), true, 0, "")
		}
		pdf.Ln(-1)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
