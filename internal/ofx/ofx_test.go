package ofx

import "testing"

// OFX 1.x (SGML), estilo dos bancos brasileiros: cabeçalho com ENCODING,
// tags-folha sem fechamento, <STMTTRN> fechado.
const extratoSGML = "OFXHEADER:100\r\n" +
	"DATA:OFXSGML\r\n" +
	"VERSION:102\r\n" +
	"ENCODING:USASCII\r\n" +
	"CHARSET:1252\r\n" +
	"\r\n" +
	"<OFX>\n" +
	"<BANKMSGSRSV1><STMTTRNRS><STMTRS>\n" +
	"<CURDEF>BRL\n" +
	"<BANKACCTFROM><BANKID>341<BRANCHID>1234<ACCTID>56789-0<ACCTTYPE>CHECKING</BANKACCTFROM>\n" +
	"<BANKTRANLIST>\n" +
	"<DTSTART>20260901000000[-3:GMT]\n" +
	"<DTEND>20260930000000[-3:GMT]\n" +
	"<STMTTRN><TRNTYPE>DEBIT<DTPOSTED>20260908120000[-3:GMT]<TRNAMT>-2400.00<FITID>AAA111<NAME>ALUGUEL IMOBILIARIA<MEMO>PAGTO ALUGUEL SET</STMTTRN>\n" +
	"<STMTTRN><TRNTYPE>CREDIT<DTPOSTED>20260910<TRNAMT>1500.00<FITID>BBB222<NAME>CLIENTE XPTO LTDA</STMTTRN>\n" +
	"<STMTTRN><TRNTYPE>FEE<DTPOSTED>20260930<TRNAMT>-1,90<FITID>CCC333<MEMO>TARIFA</STMTTRN>\n" +
	"</BANKTRANLIST>\n" +
	"<LEDGERBAL><BALAMT>-900.00<DTASOF>20260930</LEDGERBAL>\n" +
	"</STMTRS></STMTTRNRS></BANKMSGSRSV1>\n" +
	"</OFX>\n"

func TestParseSGML(t *testing.T) {
	ext, err := Parse([]byte(extratoSGML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if ext.Moeda != "BRL" {
		t.Errorf("Moeda = %q", ext.Moeda)
	}
	if ext.DataInicio != "2026-09-01" || ext.DataFim != "2026-09-30" {
		t.Errorf("período = %q a %q", ext.DataInicio, ext.DataFim)
	}
	if ext.Conta.BankID != "341" || ext.Conta.AcctID != "56789-0" || ext.Conta.AcctType != "CHECKING" {
		t.Errorf("conta = %+v", ext.Conta)
	}

	if len(ext.Transacoes) != 3 {
		t.Fatalf("esperava 3 transações, veio %d: %+v", len(ext.Transacoes), ext.Transacoes)
	}

	deb := ext.Transacoes[0]
	if deb.Tipo != "pagar" || deb.Valor != 2400.0 || deb.Data != "2026-09-08" || deb.FITID != "AAA111" {
		t.Errorf("débito = %+v", deb)
	}
	if deb.Descricao != "ALUGUEL IMOBILIARIA PAGTO ALUGUEL SET" {
		t.Errorf("descrição do débito = %q", deb.Descricao)
	}

	cred := ext.Transacoes[1]
	if cred.Tipo != "receber" || cred.Valor != 1500.0 || cred.Data != "2026-09-10" {
		t.Errorf("crédito = %+v", cred)
	}

	// vírgula decimal
	if ext.Transacoes[2].Valor != 1.90 || ext.Transacoes[2].Tipo != "pagar" {
		t.Errorf("tarifa = %+v", ext.Transacoes[2])
	}
}

// OFX 2.x (XML), com tags fechadas e encoding declarado no prólogo.
const extratoXML = `<?xml version="1.0" encoding="UTF-8"?>
<?OFX OFXHEADER="200" VERSION="220" SECURITY="NONE" OLDFILEUID="NONE" NEWFILEUID="NONE"?>
<OFX><BANKMSGSRSV1><STMTTRNRS><STMTRS>
  <CURDEF>BRL</CURDEF>
  <BANKACCTFROM><BANKID>001</BANKID><ACCTID>00099999</ACCTID><ACCTTYPE>SAVINGS</ACCTTYPE></BANKACCTFROM>
  <BANKTRANLIST>
    <DTSTART>20260101</DTSTART><DTEND>20260131</DTEND>
    <STMTTRN><TRNTYPE>PIX</TRNTYPE><DTPOSTED>20260115</DTPOSTED><TRNAMT>320.50</TRNAMT><FITID>X1</FITID><MEMO>PIX RECEBIDO</MEMO></STMTTRN>
  </BANKTRANLIST>
</STMTRS></STMTTRNRS></BANKMSGSRSV1></OFX>`

func TestParseXML(t *testing.T) {
	ext, err := Parse([]byte(extratoXML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ext.Conta.BankID != "001" || ext.Conta.AcctType != "SAVINGS" {
		t.Errorf("conta = %+v", ext.Conta)
	}
	if len(ext.Transacoes) != 1 {
		t.Fatalf("esperava 1 transação, veio %d", len(ext.Transacoes))
	}
	tr := ext.Transacoes[0]
	if tr.Tipo != "receber" || tr.Valor != 320.50 || tr.Data != "2026-01-15" || tr.Descricao != "PIX RECEBIDO" {
		t.Errorf("transação = %+v", tr)
	}
}

func TestParseLatin1(t *testing.T) {
	// "TRANSFERÊNCIA" em windows-1252 (Ê = 0xCA).
	raw := []byte("ENCODING:USASCII\r\nCHARSET:1252\r\n\r\n<OFX><BANKTRANLIST>" +
		"<STMTTRN><DTPOSTED>20260201<TRNAMT>-10.00<FITID>Z9<MEMO>TRANSFER\xcaNCIA</STMTTRN>" +
		"</BANKTRANLIST></OFX>")
	ext, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := ext.Transacoes[0].Descricao; got != "TRANSFERÊNCIA" {
		t.Errorf("descrição decodificada = %q", got)
	}
}

func TestParseNaoOFX(t *testing.T) {
	if _, err := Parse([]byte("isto não é um arquivo ofx")); err == nil {
		t.Fatal("esperava erro para conteúdo não-OFX")
	}
}
