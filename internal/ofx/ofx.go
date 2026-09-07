// Package ofx faz o parsing tolerante de arquivos OFX (Open Financial
// Exchange) de extrato bancário — tanto o formato 1.x (SGML, com tags-folha
// sem fechamento) quanto o 2.x (XML). O foco é extrair, de forma robusta
// entre bancos, o cabeçalho <BANKACCTFROM> e a lista de transações
// <STMTTRN>, já normalizados para o domínio do Fynam.
package ofx

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// Conta são os dados de identificação do <BANKACCTFROM> (ou <CCACCTFROM>).
type Conta struct {
	BankID   string
	BranchID string
	AcctID   string
	AcctType string // valores do OFX: "CHECKING", "SAVINGS", ...
}

// Transacao é uma linha <STMTTRN> já normalizada.
type Transacao struct {
	FITID     string
	Data      string  // "YYYY-MM-DD" (de <DTPOSTED>)
	Valor     float64 // sempre positivo
	Tipo      string  // "pagar" (débito, TRNAMT < 0) | "receber" (crédito)
	TrnType   string  // <TRNTYPE> cru
	Descricao string  // <NAME> + " " + <MEMO>, aparado
}

// Extrato é o resultado do parsing de um arquivo OFX.
type Extrato struct {
	Conta      Conta
	Moeda      string // <CURDEF>
	DataInicio string // "YYYY-MM-DD" (de <DTSTART>)
	DataFim    string // "YYYY-MM-DD" (de <DTEND>)
	Transacoes []Transacao
}

var (
	reTag         = regexp.MustCompile(`(?is)<([A-Za-z0-9.]+)>\s*([^<\r\n]*)`)
	reStmtTrn     = regexp.MustCompile(`(?is)<STMTTRN>(.*?)</STMTTRN>`)
	reStmtTrnAbre = regexp.MustCompile(`(?i)<STMTTRN>`)
	reAcctFrom    = regexp.MustCompile(`(?is)<(?:BANK|CC)ACCTFROM>(.*?)</(?:BANK|CC)ACCTFROM>`)
	reHeaderEnc   = regexp.MustCompile(`(?i)ENCODING:\s*([A-Za-z0-9-]+)`)
	reHeaderChar  = regexp.MustCompile(`(?i)CHARSET:\s*([A-Za-z0-9-]+)`)
	reXMLEnc      = regexp.MustCompile(`(?i)encoding=["']([A-Za-z0-9-]+)["']`)
)

// Parse lê o conteúdo bruto de um arquivo OFX e devolve o extrato
// normalizado. Erros são retornados quando o arquivo não parece ser OFX ou
// não tem nenhuma transação legível.
func Parse(raw []byte) (Extrato, error) {
	texto := decodificar(raw)

	i := indiceCaseInsensitive(texto, "<OFX>")
	if i < 0 {
		return Extrato{}, fmt.Errorf("arquivo não parece ser OFX (sem <OFX>)")
	}
	corpo := texto[i:]

	geral := folhas(corpo)
	ext := Extrato{
		Moeda:      geral["CURDEF"],
		DataInicio: dataISO(geral["DTSTART"]),
		DataFim:    dataISO(geral["DTEND"]),
	}

	if m := reAcctFrom.FindStringSubmatch(corpo); m != nil {
		f := folhas(m[1])
		ext.Conta = Conta{
			BankID:   f["BANKID"],
			BranchID: f["BRANCHID"],
			AcctID:   f["ACCTID"],
			AcctType: strings.ToUpper(f["ACCTTYPE"]),
		}
	}

	for _, bloco := range blocosDeTransacao(corpo) {
		f := folhas(bloco)
		valor, err := parseValor(f["TRNAMT"])
		if err != nil {
			continue // sem valor legível: ignora a linha
		}
		tipo := "receber"
		if valor < 0 {
			tipo = "pagar"
		}
		desc := strings.TrimSpace(f["NAME"] + " " + f["MEMO"])
		ext.Transacoes = append(ext.Transacoes, Transacao{
			FITID:     f["FITID"],
			Data:      dataISO(f["DTPOSTED"]),
			Valor:     math.Abs(valor),
			Tipo:      tipo,
			TrnType:   strings.ToUpper(f["TRNTYPE"]),
			Descricao: strings.Join(strings.Fields(desc), " "),
		})
	}

	if len(ext.Transacoes) == 0 {
		return ext, fmt.Errorf("nenhuma transação (<STMTTRN>) encontrada no arquivo")
	}
	return ext, nil
}

// blocosDeTransacao devolve o trecho de cada <STMTTRN>. Usa as tags de
// fechamento quando existem (1.x moderno e 2.x); se não, fatia por
// <STMTTRN> e corta em </BANKTRANLIST>.
func blocosDeTransacao(corpo string) []string {
	if ms := reStmtTrn.FindAllStringSubmatch(corpo, -1); len(ms) > 0 {
		out := make([]string, len(ms))
		for i, m := range ms {
			out[i] = m[1]
		}
		return out
	}
	partes := reStmtTrnAbre.Split(corpo, -1)
	if len(partes) < 2 {
		return nil
	}
	out := make([]string, 0, len(partes)-1)
	for _, p := range partes[1:] {
		if j := indiceCaseInsensitive(p, "</BANKTRANLIST>"); j >= 0 {
			p = p[:j]
		}
		out = append(out, p)
	}
	return out
}

// folhas extrai os elementos-folha (com valor) de um trecho, como um mapa
// TAG-em-maiúsculas -> valor. A primeira ocorrência de cada tag vence.
func folhas(trecho string) map[string]string {
	out := map[string]string{}
	for _, m := range reTag.FindAllStringSubmatch(trecho, -1) {
		val := strings.TrimSpace(m[2])
		if val == "" {
			continue // abertura de agregado, não folha
		}
		tag := strings.ToUpper(m[1])
		if _, existe := out[tag]; !existe {
			out[tag] = html.UnescapeString(val)
		}
	}
	return out
}

// decodificar converte os bytes para string UTF-8 conforme o cabeçalho do
// arquivo (ENCODING/CHARSET no 1.x, encoding="" no 2.x). OFX brasileiro
// costuma vir em windows-1252/ISO-8859-1.
func decodificar(raw []byte) string {
	cabecalho := raw
	if len(cabecalho) > 2048 {
		cabecalho = cabecalho[:2048]
	}
	h := string(cabecalho)

	enc := primeiroGrupo(reHeaderEnc, h)
	charset := primeiroGrupo(reHeaderChar, h)
	if x := primeiroGrupo(reXMLEnc, h); x != "" {
		enc = x
	}
	enc = strings.ToUpper(strings.TrimSpace(enc))
	charset = strings.ToUpper(strings.TrimSpace(charset))

	latin := func() string {
		if s, err := charmap.Windows1252.NewDecoder().String(string(raw)); err == nil {
			return s
		}
		return string(raw)
	}

	switch {
	case strings.Contains(enc, "UTF-8"), enc == "UTF8":
		return string(raw)
	case strings.Contains(enc, "8859"), strings.Contains(enc, "1252"),
		strings.Contains(charset, "8859"), strings.Contains(charset, "1252"):
		return latin()
	default:
		// USASCII sem charset útil, ou cabeçalho ausente: usa UTF-8 se for
		// válido; senão assume windows-1252.
		if utf8.ValidString(string(raw)) {
			return string(raw)
		}
		return latin()
	}
}

func primeiroGrupo(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

func indiceCaseInsensitive(s, sub string) int {
	return strings.Index(strings.ToUpper(s), strings.ToUpper(sub))
}

// dataISO converte <DTPOSTED>/<DTSTART> ("YYYYMMDD" com hora/fuso opcionais)
// para "YYYY-MM-DD".
func dataISO(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return ""
	}
	d := s[:8]
	for _, r := range d {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return d[:4] + "-" + d[4:6] + "-" + d[6:8]
}

// parseValor lê <TRNAMT> tolerando vírgula decimal e separador de milhar.
func parseValor(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("valor vazio")
	}
	if strings.Contains(s, ",") && !strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", ".") // vírgula decimal (alguns bancos BR)
	} else {
		s = strings.ReplaceAll(s, ",", "") // vírgula como separador de milhar
	}
	return strconv.ParseFloat(s, 64)
}
