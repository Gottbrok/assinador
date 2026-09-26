//go:build linux

package pkcs11

import "syscall"

// O filho morre com o pai: se o navegador matar o programa no meio de uma assinatura, nenhum
// processo fica com o cartão aberto.
func atributosDoFilho() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
