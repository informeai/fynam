# Fynam (protótipo)

Aplicativo desktop de gestão financeira feito com **Wails + Go**. Funcionalidades
essenciais: dashboard, contas a pagar, contas a receber, fluxo de caixa e
DRE simplificado.

## Stack

- **Wails v2** — janela desktop nativa (WebView do sistema) a partir de um
  binário Go único, para Windows, macOS e Linux
- **Go** — toda a regra de negócio (dashboard, DRE, fluxo de caixa, status
  derivado)
- **SQLite** via [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite)
  — driver em Go puro, **sem CGO**, então compila para os três sistemas sem
  toolchain C
- Persistência atrás de uma **interface agnóstica** (`internal/storage`), para
  trocar de banco no futuro sem mexer no resto do app — ver
  [Camada de persistência](#camada-de-persistência-agnóstica)
- **HTML/CSS/JS puro** na interface (`frontend/dist/`), sem frameworks e
  **sem etapa de build** — os arquivos são embutidos direto no binário
- Gráfico de entradas x saídas desenhado em `<canvas>`, sem biblioteca externa

Os dados ficam num arquivo SQLite `fynam.db` dentro da pasta de configuração
do usuário do sistema operacional:

| SO      | Caminho                                              |
| ------- | --------------------------------------------------- |
| macOS   | `~/Library/Application Support/Fynam/fynam.db` |
| Windows | `%AppData%\Fynam\fynam.db`                    |
| Linux   | `~/.config/Fynam/fynam.db`                    |

Não há nenhuma comunicação com a internet — tudo roda localmente.

## Pré-requisitos

- [Go](https://go.dev/dl/) 1.25 ou mais recente (exigência do driver SQLite;
  ver `go` em `go.mod`)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) v2.15+
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```
- Dependências de plataforma do Wails (WebView2 no Windows, `libgtk`/`libwebkit`
  no Linux; no macOS não precisa de nada extra). Rode `wails doctor` para
  conferir.

## Como rodar (modo desenvolvimento)

```bash
wails dev
```

Isso compila o Go, embute o frontend e abre a janela do Fynam com
hot-reload: alterações em `frontend/dist/` recarregam a interface na hora;
alterações no Go recompilam e reabrem o app.

## Como gerar o executável

```bash
wails build
```

O aplicativo empacotado sai em `build/bin/` (`.app` no macOS, `.exe` no
Windows, binário ELF no Linux). Para os instaladores use
`wails build -nsis` (Windows) ou `wails build -platform` cruzado conforme a
documentação do Wails.

Na primeira execução o banco já vem com uma conta "Caixa / Conta Principal"
e um plano de contas básico (categorias de receita e despesa)
pré-cadastrados, para não começar tudo vazio.

## Estrutura do projeto

```
fynam/
├── main.go              # bootstrap: resolve a pasta de dados, cria o Store SQLite, sobe o Wails
├── app.go               # App struct — métodos expostos ao frontend + regra de negócio
├── reports.go           # DTOs de saída do dashboard e dos relatórios
├── export.go            # métodos ExportarX + construtores de report.Tabela
├── update_app.go        # métodos de auto-update expostos ao frontend + eventos
├── version.go           # lê a versão do wails.json embutido
├── updater_prod.go / updater_dev.go  # liga/desliga o updater por build tag
├── seed.go              # cadastro inicial (1 conta + plano de contas)
├── helpers.go           # utilidades de data e rótulos
├── internal/
│   ├── model/           # tipos de domínio (Conta, Categoria, Lancamento, ...)
│   ├── report/          # geração de PDF / XLSX / CSV a partir de uma Tabela neutra
│   ├── updater/         # verificação e aplicação de atualização (GitHub Releases)
│   └── storage/
│       ├── storage.go   # interface Store (contrato de persistência, agnóstico)
│       └── sqlite/       # implementação SQLite da interface Store (+ testes)
├── wails.json           # configuração do Wails (sem build de frontend)
├── go.mod / go.sum
├── build/               # ícones, Info.plist, config de instalador
├── frontend/
│   ├── dist/            # interface embutida no binário (index.html, app.js, style.css)
│   ├── package.json     # placeholder — não há build
│   └── wailsjs/         # bindings TS/JS gerados pelo Wails (regenerados no build)
```

O binário Go é o único que lê/grava dados. A interface (`frontend/dist/`)
nunca acessa o disco: ela chama os métodos Go através de
`window.go.main.App.*`, que o Wails injeta em tempo de execução (os wrappers
tipados em `frontend/wailsjs/` são só conveniência para o editor).

### Camada de persistência (agnóstica)

`internal/storage` define a interface **`Store`** — CRUD puro de contas,
categorias e lançamentos, mais `SetPagamento` para baixa/estorno. Ela **não
conhece SQL**: cada backend traduz para a sua tecnologia.

```
                 ┌────────────┐   usa a interface   ┌───────────────────────┐
   frontend  ──► │ App (app.go)│ ──────────────────► │ storage.Store         │
 (window.go)     │ regra de    │                     │  (interface)          │
                 │ negócio     │                     ├───────────────────────┤
                 └────────────┘                      │ sqlite.Store  ← hoje  │
                                                     │ postgres.Store        │
                                                     │ mongo.Store           │
                                                     │ firebase.Store  ...   │
                                                     └───────────────────────┘
```

Para migrar de banco:

1. criar um pacote novo (ex.: `internal/storage/postgres`) que implemente
   `storage.Store`;
2. trocar **uma linha** em `main.go`:
   ```go
   store, err := sqlite.New(filepath.Join(dir, "fynam.db"))
   // vira, por exemplo:
   store, err := postgres.New(os.Getenv("DATABASE_URL"))
   ```

Nada em `app.go`, `reports.go`, `seed.go` ou no frontend muda. As
agregações (dashboard, DRE, fluxo de caixa) são calculadas em `app.go` sobre
os dados do `Store`, então já funcionam com qualquer backend; se algum dia o
volume exigir, elas podem descer para métodos específicos na interface.

Regras do contrato (ver comentário em `internal/storage/storage.go`):
`Create*` recebe a entidade sem id e devolve com o id atribuído; id
inexistente → `ErrNaoEncontrado`; `Status` do lançamento nunca é
persistido; remover conta/categoria **anula** a referência nos lançamentos
(não os apaga).

### Mapa da API (Go ⇄ interface)

| Método Go (`App`)                         | Uso |
| ----------------------------------------- | --- |
| `ListContas` / `CreateConta` / `UpdateConta` / `DeleteConta` | contas bancárias / caixas |
| `ListCategorias` / `CreateCategoria` / `DeleteCategoria`     | plano de contas |
| `ListLancamentos(filtro)`                 | lista contas a pagar/receber com filtros |
| `CreateLancamento` / `UpdateLancamento` / `DeleteLancamento` | CRUD de lançamentos |
| `MarcarBaixa(id, data)` / `Estornar(id)`  | baixa e estorno |
| `DashboardResumo`                         | cards, gráfico e próximos vencimentos |
| `RelatorioFluxoCaixa(ano)`               | fluxo de caixa mensal do ano |
| `RelatorioDRE(inicio, fim)`              | DRE simplificado por período |
| `ExportarFluxoCaixa(ano, formato)`       | salva o fluxo de caixa em PDF/XLSX/CSV |
| `ExportarDRE(inicio, fim, formato)`      | salva o DRE em PDF/XLSX/CSV |
| `ExportarLancamentos(filtro, formato)`   | salva a lista de lançamentos em PDF/XLSX/CSV |
| `VersaoAtual()` / `VerificarAtualizacao()` | versão em execução / checagem manual de update |
| `BaixarEAplicarAtualizacao()` / `ReiniciarApp()` | instala a atualização e relança o app |

O status de cada lançamento (`pendente`, `atrasado`, `pago`, `recebido`) é
sempre **derivado das datas** em tempo de leitura e nunca é persistido.

### Exportação de relatórios (PDF, XLSX, CSV)

As telas **Fluxo de Caixa**, **DRE**, **Contas a Pagar** e **Contas a
Receber** têm botões `Exportar: PDF · Excel · CSV`. O fluxo:

1. `app.go` monta uma `report.Tabela` neutra (título, colunas, linhas, rodapé)
   a partir do mesmo relatório que a tela mostra;
2. `internal/report` serializa essa tabela no formato pedido —
   `pdf.go` ([`go-pdf/fpdf`](https://github.com/go-pdf/fpdf), cabeçalho
   repetido por página, moeda alinhada à direita), `xlsx.go`
   ([`excelize`](https://github.com/xuri/excelize), células monetárias
   tipadas com formato `R$ #,##0.00`), `csv.go` (separador `;` + BOM UTF-8,
   abre direto no Excel pt-BR);
3. o Wails abre o diálogo nativo "Salvar como" e o Go grava o arquivo.

Para adicionar um novo relatório exportável, basta escrever um construtor
`tabelaX(...) report.Tabela` em `export.go` — os três formatos saem de graça.
Ambas as libs são Go puro (sem CGO).

### Atualização automática (GitHub Releases)

Toda vez que o app abre, uma goroutine consulta a última
[Release do repositório](https://github.com/informeai/fynam/releases) e, se
houver versão mais nova que a de `wails.json` (`info.productVersion`), emite
o evento `update:disponivel` e o frontend mostra um banner
*"Fynam X.Y.Z disponível — Ver notas · Atualizar agora · Depois"*.

- **`internal/updater`** ([`go-selfupdate`](https://github.com/creativeprojects/go-selfupdate)):
  `Verificar()` consulta a API do GitHub e escolhe o asset do SO/arquitetura;
  `Aplicar()` baixa o binário, confere o **SHA-256 contra `checksums.txt`**
  da release e substitui o executável em execução.
- **`update_app.go`**: métodos `VersaoAtual`, `VerificarAtualizacao`,
  `BaixarEAplicarAtualizacao` e `ReiniciarApp` expostos ao frontend, mais os
  eventos `update:baixando` / `update:concluido` / `update:erro`.
- Fica **desativado no `wails dev`** (build tag `dev`) e quando não há versão
  válida em `wails.json`.

**Publicar uma versão:** bump em `wails.json` → `git tag v0.3.0` →
`git push --tags`. O workflow `.github/workflows/release.yml` compila
macOS (universal) / Windows / Linux, gera `checksums.txt` e cria a Release
com os assets nos nomes que o updater espera
(`fynam_darwin_universal.tar.gz`, `fynam_windows_amd64.zip`,
`fynam_linux_amd64.tar.gz`).

> **macOS:** o updater troca só o binário `fynam.app/Contents/MacOS/fynam`.
> Funciona para o app não-assinado atual; para distribuição séria, o ideal
> é assinar com Developer ID + notarizar e trocar o `.app` inteiro. A
> validação por `checksums.txt` protege contra download corrompido, mas
> **não** é assinatura criptográfica — para isso, trocar o
> `ChecksumValidator` por um `ECDSAValidator` com chave pública embutida.

## Funcionalidades desta versão

- **Dashboard** — saldo atual, total a pagar/receber em aberto, gráfico dos
  últimos 6 meses (entradas x saídas) e lista dos próximos vencimentos.
- **Contas a Pagar / Contas a Receber** — cadastro, edição, exclusão,
  filtro por status (pendente, atrasado, pago/recebido) e baixa (marcar
  como pago/recebido, com opção de estornar).
- **Fluxo de Caixa** — visão mensal (ano selecionável) com entradas,
  saídas, saldo do mês e saldo acumulado.
- **DRE simplificado** — por período, agrupado por categoria, com receita
  bruta, despesas e resultado.
- **Exportação** — fluxo de caixa, DRE e listas de lançamentos em **PDF**,
  **Excel (.xlsx)** e **CSV**, via diálogo nativo "Salvar como".
- **Atualização automática** — verifica as Releases do GitHub a cada abertura
  e instala a versão nova com um clique.
- **Cadastros** — contas bancárias/caixas e categorias (plano de contas).

## Testes

```bash
go test ./...
```

Cobrem a implementação SQLite da interface `Store` (CRUD, filtros, baixa/
estorno, anulação de referência ao remover categoria/conta, erros de id
inexistente), a regra de negócio em `app.go` (saldo do dashboard, DRE,
fluxo de caixa acumulado e filtro por status derivado) e a geração de
relatórios em `internal/report` (formatação de moeda pt-BR e saída válida
de PDF, XLSX e CSV). Cada implementação futura de `Store` pode reaproveitar
o mesmo estilo de teste do pacote `internal/storage/sqlite`.

## O que ainda falta para virar um produto completo

- **Múltiplas filiais** e **múltiplos usuários/permissões**
- **Conciliação bancária** (importação de extrato OFX/CSV)
- **Controle de inadimplência** com régua de cobrança
- **Backup automático em nuvem** (hoje os dados ficam só na máquina local)
- **Autenticação e licenciamento**, se for vender como assinatura
- Assinatura e notarização dos instaladores para distribuição
