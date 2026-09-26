//go:build !windows

package main

import (
	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/pkcs11"
)

// No Linux e no macOS, as chaves vêm dos módulos PKCS#11, cada um num filho.
func provedores(executavel string) []assinatura.Provedor {
	return []assinatura.Provedor{pkcs11.NovoProvedor(executavel, pkcs11.OpcoesPadrao())}
}

func executarModulo(caminho string) int {
	return pkcs11.ExecutarModulo(caminho)
}
