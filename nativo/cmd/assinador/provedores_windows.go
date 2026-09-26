//go:build windows

package main

import (
	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/windows"
)

// No Windows, as chaves vêm do repositório pessoal do usuário, pelo CNG e pelo CSP (§3.5 do plano).
// A janela-mãe é o `--parent-window` do Chrome, para o diálogo de PIN abrir na frente.
func provedores(_ string, janela uint64) []assinatura.Provedor {
	return []assinatura.Provedor{windows.NovoProvedor(janela)}
}

// No Windows não há filho de módulo PKCS#11.
func executarModulo(string) int {
	return 2
}

// servicoDePropagacao é o estado do `CertPropSvc`, para o diagnóstico.
func servicoDePropagacao() string {
	return windows.EstadoDoServicoDePropagacao()
}
