package main

import (
	_ "embed"
	"encoding/json"
)

// wailsJSON é o wails.json embutido no binário em tempo de compilação, para
// que a versão do produto esteja disponível em runtime sem depender de
// ldflags.
//
//go:embed wails.json
var wailsJSON []byte

// appVersion é a versão do produto, lida de wails.json (campo
// info.productVersion). "0.0.0" quando não for possível ler — nesse caso o
// updater fica desativado.
var appVersion = lerVersao()

func lerVersao() string {
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsJSON, &cfg); err != nil || cfg.Info.ProductVersion == "" {
		return "0.0.0"
	}
	return cfg.Info.ProductVersion
}
