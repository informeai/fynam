package updater

import "testing"

func TestNewValidaVersao(t *testing.T) {
	casos := []struct {
		versao string
		ok     bool
	}{
		{"0.2.0", true},
		{"1.10.3", true},
		{"v0.3.0", true},
		{"", false},
		{"0.0.0", false},
		{"não-é-versão", false},
	}
	for _, c := range casos {
		_, err := New("informeai", "fynam", c.versao)
		if c.ok && err != nil {
			t.Errorf("New(%q): erro inesperado: %v", c.versao, err)
		}
		if !c.ok && err == nil {
			t.Errorf("New(%q): esperava erro, veio nil", c.versao)
		}
	}
}

func TestVersaoAtual(t *testing.T) {
	u, err := New("informeai", "fynam", "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if u.VersaoAtual() != "0.2.0" {
		t.Errorf("VersaoAtual() = %q, esperado 0.2.0", u.VersaoAtual())
	}
}

func TestAplicarSemAtualizacaoCarregada(t *testing.T) {
	u, _ := New("informeai", "fynam", "0.2.0")
	if err := u.Aplicar(t.Context(), nil); err == nil {
		t.Error("Aplicar(nil) deveria falhar")
	}
}
