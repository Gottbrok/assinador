//go:build !linux && !windows

package pkcs11

import "syscall"

// Fora do Linux não há Pdeathsig: se o host morrer no meio, o filho termina a operação em curso
// (que pode ser a pessoa digitando o PIN no teclado da leitora), falha ao responder pelo canal
// fechado e sai. Ele não lê mais o descritor do pedido depois do PIN, então não percebe antes. A
// F8 (macOS) decide se vale vigiar o pai.
func atributosDoFilho() *syscall.SysProcAttr {
	return nil
}
