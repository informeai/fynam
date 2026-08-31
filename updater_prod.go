//go:build !dev

package main

// updaterHabilitado controla se a verificação automática de atualização
// roda. Fica ativo em builds de produção (`wails build`) e desativado no
// modo de desenvolvimento (`wails dev`, que compila com a tag `dev`).
const updaterHabilitado = true
