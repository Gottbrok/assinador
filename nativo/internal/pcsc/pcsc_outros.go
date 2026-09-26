//go:build !linux

package pcsc

// Fora do Linux, o PC/SC entra com o provedor de cada sistema (winscard no Windows, na F6a; o
// PCSC.framework no macOS, na F8).
func Consultar() Resultado {
	return Resultado{Estado: EstadoSemBiblioteca, Detalhe: "PC/SC deste sistema ainda não implementado", Leitoras: []Leitora{}}
}
