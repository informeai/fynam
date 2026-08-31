package main

import (
	"fmt"
	"strconv"
	"time"

	"fynam/internal/model"
)

// =====================================================================
// Empresas / filiais  (gerenciamento de várias empresas)
// =====================================================================
//
// Todo o resto do app (contas, categorias, lançamentos, dashboard,
// relatórios, exportação) opera sobre a empresa ativa. TrocarEmpresa muda
// o escopo e emite o evento "empresa:trocada" para o frontend recarregar.

// ListEmpresas devolve todas as empresas cadastradas.
func (a *App) ListEmpresas() ([]model.Empresa, error) {
	return a.store.ListEmpresas(a.c())
}

// EmpresaAtiva devolve a empresa em uso no momento.
func (a *App) EmpresaAtiva() (model.Empresa, error) {
	return a.store.GetEmpresa(a.c(), a.empresa())
}

// CriarEmpresa cadastra uma nova empresa já com o plano de contas básico e
// uma conta "Caixa", e passa a trabalhar nela.
func (a *App) CriarEmpresa(nome string, cnpj string) (model.Empresa, error) {
	emp, err := a.store.CreateEmpresa(a.c(), model.Empresa{
		Nome:     nome,
		CNPJ:     cnpj,
		CriadaEm: time.Now().Format("2006-01-02"),
	})
	if err != nil {
		return model.Empresa{}, err
	}
	if err := seedEmpresa(a.c(), a.store, emp.ID); err != nil {
		return model.Empresa{}, err
	}

	a.definirEmpresaAtiva(emp.ID)
	a.emitir("empresa:trocada", emp.ID)
	return emp, nil
}

// AtualizarEmpresa altera nome/CNPJ de uma empresa.
func (a *App) AtualizarEmpresa(id int, nome string, cnpj string) (model.Empresa, error) {
	return a.store.UpdateEmpresa(a.c(), model.Empresa{ID: id, Nome: nome, CNPJ: cnpj})
}

// ExcluirEmpresa remove a empresa e, em cascata, todos os seus dados.
// Não permite excluir a última empresa. Se a excluída for a ativa, passa a
// trabalhar em outra.
func (a *App) ExcluirEmpresa(id int) error {
	empresas, err := a.store.ListEmpresas(a.c())
	if err != nil {
		return err
	}
	if len(empresas) <= 1 {
		return fmt.Errorf("não é possível excluir a única empresa")
	}

	if err := a.store.DeleteEmpresa(a.c(), id); err != nil {
		return err
	}

	if a.empresa() == id {
		for _, e := range empresas {
			if e.ID != id {
				a.definirEmpresaAtiva(e.ID)
				a.emitir("empresa:trocada", e.ID)
				break
			}
		}
	}
	return nil
}

// TrocarEmpresa muda a empresa ativa. O frontend deve recarregar tudo.
func (a *App) TrocarEmpresa(id int) error {
	if _, err := a.store.GetEmpresa(a.c(), id); err != nil {
		return err
	}
	a.definirEmpresaAtiva(id)
	a.emitir("empresa:trocada", id)
	return nil
}

// definirEmpresaAtiva atualiza o estado em memória e persiste em config.
func (a *App) definirEmpresaAtiva(id int) {
	a.mu.Lock()
	a.empresaAtivaID = id
	a.mu.Unlock()
	_ = a.store.SetConfig(a.c(), chaveEmpresaAtiva, strconv.Itoa(id))
}
