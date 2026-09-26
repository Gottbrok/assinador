//go:build windows

package main

import "os"

// No Windows o host não carrega biblioteca em C no próprio processo (o provedor é CNG e CSP, pela
// API do sistema); a F6a decide se o canal precisa da mesma separação.
func separarCanal() (*os.File, error) {
	return os.Stdout, nil
}
