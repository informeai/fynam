package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"fynam/internal/storage/sqlite"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dir, err := appDataDir()
	if err != nil {
		log.Fatalf("não foi possível resolver a pasta de dados: %v", err)
	}

	// Troque esta linha para migrar de backend de persistência no futuro
	// (ex.: postgres.New(dsn), mongo.New(uri), ...). O resto do app não muda.
	store, err := sqlite.New(filepath.Join(dir, "fynam.db"))
	if err != nil {
		log.Fatalf("não foi possível abrir o banco local: %v", err)
	}
	defer store.Close()

	if err := seedIfEmpty(context.Background(), store, caminhoDadosLegado(dir)); err != nil {
		log.Fatalf("falha ao preparar os dados iniciais: %v", err)
	}

	app := NewApp(store)

	err = wails.Run(&options.App{
		Title:     "Fynam",
		Width:     1280,
		Height:    800,
		MinWidth:  1024,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 244, G: 246, B: 249, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatalf("erro ao executar a aplicação: %v", err)
	}
}

// appDataDir devolve (criando se preciso) a pasta de dados do Fynam dentro
// do diretório de configuração do usuário do sistema operacional.
func appDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "Fynam")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// caminhoDadosLegado aponta para o xfin-data.json da versão anterior
// (Electron/lowdb ou primeira iteração Wails), para importação automática
// na primeira execução. Procura na pasta nova e, se não achar, na antiga.
func caminhoDadosLegado(dir string) string {
	atual := filepath.Join(dir, "xfin-data.json")
	if _, err := os.Stat(atual); err == nil {
		return atual
	}
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, "XFin Desktop", "xfin-data.json")
	}
	return atual
}
