package main

import "fynam/internal/model"

// DTOs de saída dos relatórios e do dashboard.
//
// São agregações calculadas na camada de aplicação (app.go) a partir das
// entidades devolvidas pelo Store — não são persistidos e não fazem parte
// da interface storage.Store.

// ----- Dashboard -----

type FluxoMes struct {
	Mes      string  `json:"mes"`
	Entradas float64 `json:"entradas"`
	Saidas   float64 `json:"saidas"`
}

type Resumo struct {
	SaldoAtual          float64            `json:"saldoAtual"`
	TotalAPagar         float64            `json:"totalAPagar"`
	TotalAReceber       float64            `json:"totalAReceber"`
	ProximosVencimentos []model.Lancamento `json:"proximosVencimentos"`
	FluxoMensal         []FluxoMes         `json:"fluxoMensal"`
	Hoje                string             `json:"hoje"`
}

// ----- Relatórios -----

type FluxoCaixaLinha struct {
	Mes            string  `json:"mes"`
	Entradas       float64 `json:"entradas"`
	Saidas         float64 `json:"saidas"`
	SaldoPeriodo   float64 `json:"saldoPeriodo"`
	SaldoAcumulado float64 `json:"saldoAcumulado"`
}

type DRELinha struct {
	Categoria string  `json:"categoria"`
	Tipo      string  `json:"tipo"`
	Total     float64 `json:"total"`
}

type DRE struct {
	Linhas       []DRELinha `json:"linhas"`
	ReceitaBruta float64    `json:"receitaBruta"`
	Despesas     float64    `json:"despesas"`
	Resultado    float64    `json:"resultado"`
}
