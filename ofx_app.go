package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"fynam/internal/model"
	"fynam/internal/ofx"
)

// =====================================================================
// Importação e conciliação de extrato OFX (v1)
// =====================================================================
//
// Fluxo em dois passos:
//  1. ImportarExtratoOFX abre o seletor de arquivo, parseia o OFX e devolve
//     uma prévia: cada linha do extrato já com o casamento automático
//     contra os lançamentos da conta (0, 1 ou vários candidatos).
//  2. AplicarImportacaoOFX efetiva as decisões do operador (conciliar com
//     um lançamento existente, criar um lançamento novo, ou ignorar).
//
// O casamento é sempre escopado por conta e por tipo: débito do extrato
// (TRNAMT < 0) casa com "pagar"; crédito, com "receber".

// ofxToleranciaDias é a janela (em dias) entre a data do extrato e a data
// de vencimento/pagamento do lançamento para considerá-los candidatos.
const ofxToleranciaDias = 5

// LinhaConciliacao é uma linha do extrato mais o resultado do casamento
// automático.
type LinhaConciliacao struct {
	Linha       model.ExtratoLinha `json:"linha"`
	JaImportada bool               `json:"jaImportada"`
	Candidatos  []model.Lancamento `json:"candidatos"`
	Sugestao    string             `json:"sugestao"` // "conciliar" | "criar" | "ignorar"
	SugestaoID  *int               `json:"sugestaoId"`
}

// PreviaImportacaoOFX é o retorno de ImportarExtratoOFX.
type PreviaImportacaoOFX struct {
	Arquivo     string             `json:"arquivo"`
	ContaID     int                `json:"contaId"`
	SemConflito bool               `json:"semConflito"` // BANKACCTFROM não conflita com a conta
	Aviso       string             `json:"aviso"`
	Periodo     string             `json:"periodo"`
	Linhas      []LinhaConciliacao `json:"linhas"`
}

// DecisaoConciliacao é a escolha do operador para uma linha do extrato.
type DecisaoConciliacao struct {
	Linha        model.ExtratoLinha `json:"linha"`
	Acao         string             `json:"acao"` // "conciliar" | "criar" | "ignorar"
	LancamentoID *int               `json:"lancamentoId"`
	CategoriaID  *int               `json:"categoriaId"`
}

// ResumoImportacaoOFX é o retorno de AplicarImportacaoOFX.
type ResumoImportacaoOFX struct {
	Conciliados int      `json:"conciliados"`
	Criados     int      `json:"criados"`
	Ignorados   int      `json:"ignorados"`
	Erros       []string `json:"erros"`
}

// ImportarExtratoOFX abre o seletor de arquivo, parseia o OFX e devolve a
// prévia da conciliação contra os lançamentos da conta informada. Se o
// operador cancelar o diálogo, devolve uma prévia vazia (Arquivo == "").
func (a *App) ImportarExtratoOFX(contaID int) (PreviaImportacaoOFX, error) {
	conta, err := a.store.GetConta(a.c(), contaID)
	if err != nil {
		return PreviaImportacaoOFX{}, err
	}

	caminho, err := wruntime.OpenFileDialog(a.c(), wruntime.OpenDialogOptions{
		Title: "Selecionar extrato OFX",
		Filters: []wruntime.FileFilter{
			{DisplayName: "Extrato OFX (*.ofx;*.qfx)", Pattern: "*.ofx;*.qfx"},
		},
	})
	if err != nil {
		return PreviaImportacaoOFX{}, err
	}
	if caminho == "" {
		return PreviaImportacaoOFX{}, nil // cancelou
	}

	raw, err := os.ReadFile(caminho)
	if err != nil {
		return PreviaImportacaoOFX{}, err
	}
	ext, err := ofx.Parse(raw)
	if err != nil {
		return PreviaImportacaoOFX{}, fmt.Errorf("não foi possível ler o extrato: %w", err)
	}

	daConta, err := a.lancamentosDaConta(contaID)
	if err != nil {
		return PreviaImportacaoOFX{}, err
	}
	registros, err := a.store.ExtratoRegistrosPorFitid(a.c(), contaID)
	if err != nil {
		return PreviaImportacaoOFX{}, err
	}

	previa := montarPreviaOFX(ext, conta, daConta, registros)
	previa.Arquivo = filepath.Base(caminho)
	previa.ContaID = contaID
	return previa, nil
}

// AplicarImportacaoOFX efetiva as decisões tomadas na tela de prévia.
// Continua nas linhas seguintes mesmo quando uma falha, acumulando os
// erros no resumo.
func (a *App) AplicarImportacaoOFX(contaID int, decisoes []DecisaoConciliacao) (ResumoImportacaoOFX, error) {
	if _, err := a.store.GetConta(a.c(), contaID); err != nil {
		return ResumoImportacaoOFX{}, err
	}

	var r ResumoImportacaoOFX
	for _, d := range decisoes {
		switch d.Acao {
		case "conciliar":
			if err := a.conciliarComExistente(contaID, d); err != nil {
				r.Erros = append(r.Erros, erroLinhaOFX(d.Linha, err))
				continue
			}
			r.Conciliados++

		case "criar":
			if err := a.criarDeLinhaOFX(contaID, d); err != nil {
				r.Erros = append(r.Erros, erroLinhaOFX(d.Linha, err))
				continue
			}
			r.Criados++

		default: // "ignorar"
			_ = a.store.RegistrarExtratoLinha(a.c(), contaID, d.Linha, "ignorado", nil)
			r.Ignorados++
		}
	}
	return r, nil
}

func (a *App) conciliarComExistente(contaID int, d DecisaoConciliacao) error {
	if d.LancamentoID == nil {
		return errors.New("sem lançamento selecionado para conciliar")
	}
	l, err := a.store.GetLancamento(a.c(), *d.LancamentoID)
	if err != nil {
		return err
	}
	if l.ContaID == nil || *l.ContaID != contaID {
		return errors.New("o lançamento pertence a outra conta")
	}
	if l.DataPagamento == "" {
		if _, err := a.store.SetPagamento(a.c(), l.ID, d.Linha.Data); err != nil {
			return err
		}
	}
	if _, err := a.store.SetConciliacao(a.c(), l.ID, d.Linha.Data); err != nil {
		return err
	}
	return a.store.RegistrarExtratoLinha(a.c(), contaID, d.Linha, "conciliado", &l.ID)
}

func (a *App) criarDeLinhaOFX(contaID int, d DecisaoConciliacao) error {
	descricao := d.Linha.Descricao
	if descricao == "" {
		descricao = "Importado do extrato"
	}
	novo, err := a.CreateLancamento(model.LancamentoInput{
		Tipo:           d.Linha.Tipo,
		Descricao:      descricao,
		CategoriaID:    d.CategoriaID,
		ContaID:        &contaID,
		Valor:          d.Linha.Valor,
		DataVencimento: d.Linha.Data,
		DataPagamento:  d.Linha.Data,
	})
	if err != nil {
		return err
	}
	if _, err := a.store.SetConciliacao(a.c(), novo.ID, d.Linha.Data); err != nil {
		return err
	}
	return a.store.RegistrarExtratoLinha(a.c(), contaID, d.Linha, "criado", &novo.ID)
}

// lancamentosDaConta devolve, já com Status derivado, os lançamentos da
// empresa ativa que pertencem à conta informada.
func (a *App) lancamentosDaConta(contaID int) ([]model.Lancamento, error) {
	brutos, err := a.store.ListLancamentos(a.c(), a.empresa(), model.LancamentoFiltro{})
	if err != nil {
		return nil, err
	}
	out := make([]model.Lancamento, 0)
	for _, l := range brutos {
		if l.ContaID != nil && *l.ContaID == contaID {
			out = append(out, l.ComStatus())
		}
	}
	return out, nil
}

// montarPreviaOFX é a parte pura (sem I/O) da importação: casa as
// transações do extrato com os lançamentos da conta e decide a sugestão.
//
// Um FITID já importado só é tratado como "já importada" enquanto o efeito
// ainda existe: linha ignorada, ou linha cujo lançamento vinculado continua
// conciliado. Se o operador desconciliou (ou excluiu) esse lançamento, a
// linha volta a ser oferecida para reconciliação.
func montarPreviaOFX(ext ofx.Extrato, conta model.Conta, daConta []model.Lancamento, registros map[string]model.ExtratoRegistro) PreviaImportacaoOFX {
	previa := PreviaImportacaoOFX{
		SemConflito: !contasConflitam(conta, ext.Conta),
		Periodo:     periodoBR(ext.DataInicio, ext.DataFim),
	}
	if !previa.SemConflito {
		previa.Aviso = fmt.Sprintf(
			"O extrato é da conta %s/%s e a conta selecionada tem %s/%s cadastrados. Confira antes de aplicar.",
			naoVazio(ext.Conta.BankID), naoVazio(ext.Conta.AcctID),
			naoVazio(conta.BankID), naoVazio(conta.AcctID))
	}

	porID := make(map[int]model.Lancamento, len(daConta))
	for _, l := range daConta {
		porID[l.ID] = l
	}

	usados := map[int]bool{}
	for _, t := range ext.Transacoes {
		linha := model.ExtratoLinha{
			FITID:     t.FITID,
			Data:      t.Data,
			Valor:     t.Valor,
			Tipo:      t.Tipo,
			Descricao: t.Descricao,
		}
		lc := LinhaConciliacao{Linha: linha}

		if reg, vista := registros[t.FITID]; t.FITID != "" && vista && efeitoAindaVale(reg, porID) {
			lc.JaImportada = true
			lc.Sugestao = "ignorar"
			previa.Linhas = append(previa.Linhas, lc)
			continue
		}

		lc.Candidatos = candidatosOFX(linha, daConta, usados)
		switch len(lc.Candidatos) {
		case 1:
			id := lc.Candidatos[0].ID
			lc.Sugestao, lc.SugestaoID = "conciliar", &id
			usados[id] = true
		case 0:
			lc.Sugestao = "criar"
		default:
			lc.Sugestao = "ignorar" // ambíguo: operador decide
		}
		previa.Linhas = append(previa.Linhas, lc)
	}
	return previa
}

// efeitoAindaVale diz se uma linha já importada ainda "conta": ignorada
// permanece ignorada; conciliada/criada só enquanto o lançamento vinculado
// existir e continuar conciliado.
func efeitoAindaVale(reg model.ExtratoRegistro, porID map[int]model.Lancamento) bool {
	if reg.Status == "ignorado" {
		return true
	}
	if reg.LancamentoID == nil {
		return false // lançamento vinculado foi excluído
	}
	l, ok := porID[*reg.LancamentoID]
	return ok && l.DataConciliacao != ""
}

// candidatosOFX devolve os lançamentos da conta que casam com a linha do
// extrato (mesmo tipo, valor igual, data dentro da tolerância, ainda não
// conciliados e ainda não usados por outra linha), ordenados pela
// proximidade de data.
func candidatosOFX(linha model.ExtratoLinha, lancs []model.Lancamento, usados map[int]bool) []model.Lancamento {
	var out []model.Lancamento
	for _, l := range lancs {
		if usados[l.ID] || l.Tipo != linha.Tipo || l.DataConciliacao != "" {
			continue
		}
		if math.Abs(l.Valor-linha.Valor) > 0.005 {
			continue
		}
		if diasEntre(refDataOFX(l), linha.Data) > ofxToleranciaDias {
			continue
		}
		out = append(out, l)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return diasEntre(refDataOFX(out[i]), linha.Data) < diasEntre(refDataOFX(out[j]), linha.Data)
	})
	return out
}

func refDataOFX(l model.Lancamento) string {
	if l.DataPagamento != "" {
		return l.DataPagamento
	}
	return l.DataVencimento
}

// contasConflitam só afirma conflito quando os dois lados têm número de
// conta e eles diferem (comparando apenas os dígitos).
func contasConflitam(c model.Conta, o ofx.Conta) bool {
	if c.AcctID == "" || o.AcctID == "" {
		return false
	}
	return soDigitos(c.AcctID) != soDigitos(o.AcctID)
}

func soDigitos(s string) string {
	var b []rune
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b = append(b, r)
		}
	}
	return string(b)
}

func naoVazio(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// diasEntre devolve a diferença absoluta em dias entre duas datas
// "YYYY-MM-DD". Se alguma não parseia, devolve um valor grande.
func diasEntre(a, b string) int {
	ta, ea := time.Parse("2006-01-02", a)
	tb, eb := time.Parse("2006-01-02", b)
	if ea != nil || eb != nil {
		return 1 << 30
	}
	d := int(ta.Sub(tb).Hours() / 24)
	if d < 0 {
		return -d
	}
	return d
}

func periodoBR(inicio, fim string) string {
	switch {
	case inicio != "" && fim != "":
		return dataBR(inicio) + " a " + dataBR(fim)
	case inicio != "":
		return "a partir de " + dataBR(inicio)
	case fim != "":
		return "até " + dataBR(fim)
	default:
		return ""
	}
}

func erroLinhaOFX(l model.ExtratoLinha, err error) string {
	ref := l.Descricao
	if ref == "" {
		ref = l.FITID
	}
	return fmt.Sprintf("%s (%s): %v", ref, dataBR(l.Data), err)
}
