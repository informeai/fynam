package report

import (
	"bytes"
	"encoding/csv"
)

// bomUTF8 é o Byte Order Mark que faz o Excel abrir o CSV como UTF-8
// (senão os acentos ficam corrompidos ao abrir com duplo clique).
var bomUTF8 = []byte{0xEF, 0xBB, 0xBF}

// gerarCSV produz um CSV com ";" como separador (padrão do Excel em pt-BR).
func gerarCSV(t Tabela) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(bomUTF8)

	w := csv.NewWriter(&buf)
	w.Comma = ';'

	if t.Titulo != "" {
		_ = w.Write([]string{t.Titulo})
	}
	if t.Subtitulo != "" {
		_ = w.Write([]string{t.Subtitulo})
	}
	if t.Titulo != "" || t.Subtitulo != "" {
		_ = w.Write([]string{""})
	}

	cab := make([]string, len(t.Colunas))
	for i, c := range t.Colunas {
		cab[i] = c.Titulo
	}
	_ = w.Write(cab)

	for _, ln := range t.Linhas {
		if err := w.Write(textos(ln)); err != nil {
			return nil, err
		}
	}
	for _, r := range t.Rodape {
		if err := w.Write(textos(r)); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
