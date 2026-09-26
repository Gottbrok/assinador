//go:build !linux && !windows

package pcsc

// No macOS, o PC/SC entra com o PCSC.framework, na F8.
func Consultar() Resultado {
	return Resultado{Estado: EstadoSemBiblioteca, Detalhe: "PC/SC deste sistema ainda não implementado", Leitoras: []Leitora{}}
}
