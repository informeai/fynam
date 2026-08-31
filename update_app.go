package main

import (
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"fynam/internal/updater"
)

// =====================================================================
// Auto-update (GitHub Releases)
// =====================================================================
//
// Fluxo:
//   1. no startup, uma goroutine chama checarAtualizacaoNoInicio();
//   2. se houver versão mais nova, emite o evento "update:disponivel"
//      com os dados da release; o frontend mostra um banner;
//   3. o usuário aceita → BaixarEAplicarAtualizacao() baixa o binário,
//      confere o checksum e substitui o executável (emite "update:baixando",
//      "update:concluido" ou "update:erro");
//   4. ReiniciarApp() relança o Fynam já atualizado.
//
// Eventos emitidos para o frontend:
//   update:disponivel  {versaoAtual, versaoNova, notas, url, publicada}
//   update:baixando    (sem payload)
//   update:concluido   {versaoNova}
//   update:erro        "mensagem"

const atrasoChecagemInicial = 3 * time.Second

// checarAtualizacaoNoInicio roda em goroutine no startup.
func (a *App) checarAtualizacaoNoInicio() {
	time.Sleep(atrasoChecagemInicial) // deixa a interface carregar primeiro

	at, err := a.buscarAtualizacao()
	if err != nil {
		log.Printf("verificação de atualização falhou (ignorando): %v", err)
		return
	}
	if at != nil {
		a.emitir("update:disponivel", at)
	}
}

// buscarAtualizacao consulta o GitHub e guarda o resultado em a.ultimaAtualizacao.
func (a *App) buscarAtualizacao() (*updater.Atualizacao, error) {
	if a.upd == nil {
		return nil, nil
	}
	at, err := a.upd.Verificar(a.c())
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.ultimaAtualizacao = at
	a.mu.Unlock()
	return at, nil
}

// -------- métodos expostos ao frontend --------

// VersaoAtual devolve a versão em execução (de wails.json).
func (a *App) VersaoAtual() string {
	return appVersion
}

// VerificarAtualizacao força uma checagem imediata. Devolve nil se já
// estiver na última versão, se o updater estiver desativado, ou em erro de
// rede (nesse caso o erro também é retornado).
func (a *App) VerificarAtualizacao() (*updater.Atualizacao, error) {
	if a.upd == nil {
		return nil, nil
	}
	return a.buscarAtualizacao()
}

// BaixarEAplicarAtualizacao baixa e instala a última atualização detectada.
// Não reinicia o app — chame ReiniciarApp() depois.
func (a *App) BaixarEAplicarAtualizacao() error {
	a.mu.Lock()
	at := a.ultimaAtualizacao
	a.mu.Unlock()

	if a.upd == nil || at == nil {
		return nil
	}

	a.emitir("update:baixando")
	if err := a.upd.Aplicar(a.c(), at); err != nil {
		a.emitir("update:erro", err.Error())
		return err
	}
	a.emitir("update:concluido", map[string]string{"versaoNova": at.VersaoNova})
	return nil
}

// ReiniciarApp relança o executável (já atualizado) e fecha a instância atual.
func (a *App) ReiniciarApp() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	// No macOS, se o binário está dentro de um .app, reabrir o bundle.
	if runtime.GOOS == "darwin" {
		if i := strings.Index(exe, ".app/Contents/MacOS/"); i != -1 {
			bundle := exe[:i+len(".app")]
			if err := exec.Command("/usr/bin/open", "-n", bundle).Start(); err != nil {
				return err
			}
			wruntime.Quit(a.c())
			return nil
		}
	}

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}
	wruntime.Quit(a.c())
	return nil
}
