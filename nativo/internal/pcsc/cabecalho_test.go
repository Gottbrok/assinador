//go:build (linux || windows) && pcsc_cabecalho

package pcsc

import "testing"

// Os tipos e constantes de `tipos.h` e de `pcsc.go` são os do pcsc-lite desta plataforma. Roda com
// os cabeçalhos de desenvolvimento (`libpcsclite-dev`, `pcsc-lite-devel`); o `winscard.h` inclui
// `pcsclite.h` sem o `PCSC/`, então a pasta dele vai no caminho:
//
//	CGO_CFLAGS="$(pkg-config --cflags libpcsclite)" go test -tags pcsc_cabecalho ./internal/pcsc/
//
// No Windows, a estrutura de `pcsc_windows.go` e as constantes são as do `winscard.h` do SDK (o do
// MinGW, no CI), com o cgo ligado só para o teste: `CGO_ENABLED=1 go test -tags pcsc_cabecalho`.
func TestTiposBatemComOCabecalho(t *testing.T) {
	for nome, par := range comparacaoComOCabecalho() {
		if par[0] != par[1] {
			t.Errorf("%s: o cabeçalho diz %d, nós %d", nome, par[0], par[1])
		}
	}
}
