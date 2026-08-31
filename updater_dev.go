//go:build dev

package main

// Em modo de desenvolvimento (`wails dev`) a verificação de atualização
// fica desligada — não faz sentido um build local tentar se atualizar.
const updaterHabilitado = false
