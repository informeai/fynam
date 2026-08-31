package main

import (
	"strconv"
	"time"
)

// todayISO devolve a data de hoje no formato "YYYY-MM-DD" (hora local).
func todayISO() string {
	return time.Now().Format("2006-01-02")
}

var mesesAbrev = [...]string{
	"Jan", "Fev", "Mar", "Abr", "Mai", "Jun",
	"Jul", "Ago", "Set", "Out", "Nov", "Dez",
}

// rotuloMes converte "YYYY-MM" em "Ago/2026".
func rotuloMes(chave string) string {
	if len(chave) < 7 {
		return chave
	}
	m, err := strconv.Atoi(chave[5:7])
	if err != nil || m < 1 || m > 12 {
		return chave
	}
	return mesesAbrev[m-1] + "/" + chave[:4]
}

// dataBR converte "YYYY-MM-DD" em "DD/MM/YYYY".
func dataBR(iso string) string {
	if len(iso) < 10 {
		return iso
	}
	return iso[8:10] + "/" + iso[5:7] + "/" + iso[:4]
}

// rotuloStatus devolve o rótulo legível de um status derivado.
func rotuloStatus(status string) string {
	switch status {
	case "pendente":
		return "Pendente"
	case "atrasado":
		return "Atrasado"
	case "pago":
		return "Pago"
	case "recebido":
		return "Recebido"
	default:
		return status
	}
}

// mesAno extrai "YYYY-MM" de uma data "YYYY-MM-DD".
func mesAno(dataISO string) string {
	if len(dataISO) < 7 {
		return dataISO
	}
	return dataISO[:7]
}

// ultimosMeses devolve os últimos `qtd` meses como "YYYY-MM", em ordem
// cronológica (o último elemento é o mês atual).
func ultimosMeses(qtd int) []string {
	out := make([]string, 0, qtd)
	now := time.Now()
	for i := qtd - 1; i >= 0; i-- {
		t := time.Date(now.Year(), now.Month()-time.Month(i), 1, 0, 0, 0, 0, time.Local)
		out = append(out, t.Format("2006-01"))
	}
	return out
}

// monthKey formata "YYYY-MM" a partir de ano e mês numéricos.
func monthKey(ano, mes int) string {
	return time.Date(ano, time.Month(mes), 1, 0, 0, 0, 0, time.Local).Format("2006-01")
}
