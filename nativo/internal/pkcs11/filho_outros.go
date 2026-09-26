//go:build !linux && !windows

package pkcs11

import "syscall"

// Fora do Linux não há Pdeathsig; o filho sai quando o descritor do pedido fecha.
func atributosDoFilho() *syscall.SysProcAttr {
	return nil
}
