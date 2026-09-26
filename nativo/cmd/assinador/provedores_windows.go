//go:build windows

package main

import "github.com/Gottbrok/assinador/nativo/internal/assinatura"

// O provedor do Windows (CNG e CSP) entra na F6a. Até lá o programa no Windows lista nada.
func provedores(string) []assinatura.Provedor {
	return nil
}

// No Windows não há filho de módulo PKCS#11.
func executarModulo(string) int {
	return 2
}
