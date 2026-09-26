//go:build windows && windows_cabecalho

package windows

import "testing"

// Os códigos, as constantes e as estruturas que o provedor escreve à mão são os do SDK do Windows
// (o do MinGW, no CI), com o cgo ligado só para o teste:
//
//	CGO_ENABLED=1 go test -tags windows_cabecalho ./internal/windows/
func TestTiposBatemComOCabecalho(t *testing.T) {
	for nome, par := range comparacaoComOCabecalho() {
		if par[0] != par[1] {
			t.Errorf("%s: o cabeçalho diz %d (0x%X), nós %d (0x%X)", nome, par[0], par[0], par[1], par[1])
		}
	}
}
