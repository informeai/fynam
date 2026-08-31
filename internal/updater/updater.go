// Package updater implementa a verificação e a aplicação de atualizações
// do Fynam a partir das Releases do GitHub.
//
// Usa github.com/creativeprojects/go-selfupdate: consulta a última release
// via API do GitHub, escolhe o asset do SO/arquitetura em execução, baixa,
// confere o SHA-256 contra o arquivo `checksums.txt` da release e substitui
// o executável atual. O reinício do app é responsabilidade de quem chama.
//
// Para a atualização funcionar, cada Release precisa conter:
//   - um asset por plataforma nomeado com o SO e a arquitetura
//     (ex.: `fynam_darwin_universal.tar.gz`, `fynam_windows_amd64.zip`,
//     `fynam_linux_amd64.tar.gz`), com o binário `fynam`/`fynam.exe` dentro;
//   - um asset `checksums.txt` com os SHA-256 de todos os outros assets.
package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/hashicorp/go-version"
)

// Atualizacao descreve uma versão mais recente disponível.
type Atualizacao struct {
	VersaoAtual string    `json:"versaoAtual"`
	VersaoNova  string    `json:"versaoNova"`
	Notas       string    `json:"notas"`
	URL         string    `json:"url"` // página da release no GitHub
	Publicada   time.Time `json:"publicada"`

	rel *selfupdate.Release // guardado para o passo Aplicar
}

// Updater consulta e aplica atualizações de um repositório do GitHub.
type Updater struct {
	repo        selfupdate.RepositorySlug
	versaoAtual string
	up          *selfupdate.Updater
}

// New cria um Updater para github.com/<owner>/<repo>. versaoAtual precisa
// ser um semver válido e diferente de "0.0.0" (build sem versão).
func New(owner, repo, versaoAtual string) (*Updater, error) {
	if versaoAtual == "" || versaoAtual == "0.0.0" {
		return nil, fmt.Errorf("versão atual ausente (%q) — updater desativado", versaoAtual)
	}
	if _, err := version.NewSemver(versaoAtual); err != nil {
		return nil, fmt.Errorf("versão atual inválida %q: %w", versaoAtual, err)
	}

	up, err := selfupdate.NewUpdater(selfupdate.Config{
		// Confere o SHA-256 do asset baixado contra checksums.txt da release.
		Validator:     &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
		UniversalArch: "universal", // asset macOS universal quando não há arch-específico
	})
	if err != nil {
		return nil, err
	}

	return &Updater{
		repo:        selfupdate.NewRepositorySlug(owner, repo),
		versaoAtual: versaoAtual,
		up:          up,
	}, nil
}

// VersaoAtual devolve a versão em execução.
func (u *Updater) VersaoAtual() string { return u.versaoAtual }

// Verificar consulta a última release. Devolve:
//   - (*Atualizacao, nil) se houver versão mais nova;
//   - (nil, nil) se já estiver atualizado ou não houver releases;
//   - (nil, err) em falha de rede/API.
func (u *Updater) Verificar(ctx context.Context) (*Atualizacao, error) {
	rel, encontrada, err := u.up.DetectLatest(ctx, u.repo)
	if err != nil {
		return nil, err
	}
	if !encontrada || rel == nil || rel.Version() == "" {
		return nil, nil
	}
	if !rel.GreaterThan(u.versaoAtual) {
		return nil, nil
	}
	return &Atualizacao{
		VersaoAtual: u.versaoAtual,
		VersaoNova:  rel.Version(),
		Notas:       rel.ReleaseNotes,
		URL:         rel.URL,
		Publicada:   rel.PublishedAt,
		rel:         rel,
	}, nil
}

// Aplicar baixa o binário da atualização, valida o checksum e substitui o
// executável atual. NÃO reinicia o app.
func (u *Updater) Aplicar(ctx context.Context, a *Atualizacao) error {
	if a == nil || a.rel == nil {
		return fmt.Errorf("nenhuma atualização carregada — chame Verificar antes")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolvido, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolvido
	}
	return u.up.UpdateTo(ctx, a.rel, exe)
}
