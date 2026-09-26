//go:build !windows

package main

import (
	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/pkcs11"
)

// No Linux e no macOS, as chaves vêm dos módulos PKCS#11, cada um num filho. A janela-mãe é do
// Windows (o `--parent-window` do Chrome): aqui o PIN é digitado na janela da extensão.
func provedores(executavel string, _ uint64) []assinatura.Provedor {
	return []assinatura.Provedor{pkcs11.NovoProvedor(executavel, pkcs11.OpcoesPadrao())}
}

// servicoDePropagacao é o `CertPropSvc` do Windows: fora dele, não se aplica.
func servicoDePropagacao() string {
	return ""
}

func executarModulo(caminho string) int {
	return pkcs11.ExecutarModulo(caminho)
}
