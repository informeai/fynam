package report

import (
	"github.com/xuri/excelize/v2"
)

// gerarXLSX produz uma planilha .xlsx com título, cabeçalho destacado,
// células monetárias tipadas (formato "R$ #,##0.00") e rodapé em negrito.
func gerarXLSX(t Tabela) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const aba = "Relatório"
	idx, err := f.NewSheet(aba)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	_ = f.DeleteSheet("Sheet1")

	numFmt := "\"R$ \"#,##0.00"
	estiloTitulo, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})
	estiloSub, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Size: 10, Color: "64748B"}})
	estiloCab, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E2E8F0"}},
	})
	estiloMoeda, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &numFmt})
	estiloMoedaBold, _ := f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true},
		CustomNumFmt: &numFmt,
	})
	estiloBold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})

	linha := 1
	cel := func(col, row int) string {
		name, _ := excelize.CoordinatesToCellName(col, row)
		return name
	}

	if t.Titulo != "" {
		c := cel(1, linha)
		f.SetCellValue(aba, c, t.Titulo)
		f.SetCellStyle(aba, c, c, estiloTitulo)
		linha++
	}
	if t.Subtitulo != "" {
		c := cel(1, linha)
		f.SetCellValue(aba, c, t.Subtitulo)
		f.SetCellStyle(aba, c, c, estiloSub)
		linha++
	}
	if linha > 1 {
		linha++ // linha em branco
	}

	for i, col := range t.Colunas {
		c := cel(i+1, linha)
		f.SetCellValue(aba, c, col.Titulo)
		f.SetCellStyle(aba, c, c, estiloCab)
	}
	linha++

	escrever := func(cells []Celula, negrito bool) {
		for i, cl := range cells {
			c := cel(i+1, linha)
			if cl.Valor != nil {
				f.SetCellFloat(aba, c, *cl.Valor, 2, 64)
				if negrito {
					f.SetCellStyle(aba, c, c, estiloMoedaBold)
				} else {
					f.SetCellStyle(aba, c, c, estiloMoeda)
				}
			} else {
				f.SetCellValue(aba, c, cl.Texto)
				if negrito {
					f.SetCellStyle(aba, c, c, estiloBold)
				}
			}
		}
		linha++
	}

	for _, ln := range t.Linhas {
		escrever(ln, false)
	}
	for _, r := range t.Rodape {
		escrever(r, true)
	}

	for i, col := range t.Colunas {
		nome, _ := excelize.ColumnNumberToName(i + 1)
		largura := 16.0
		if col.Alinhar == Esquerda {
			largura = 30.0
		}
		_ = f.SetColWidth(aba, nome, nome, largura)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
