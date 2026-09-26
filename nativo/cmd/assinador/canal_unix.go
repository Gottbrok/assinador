//go:build !windows

package main

import (
	"os"
	"syscall"
)

// separarCanal guarda uma CÓPIA do descritor da saída padrão para o canal com a extensão e aponta o
// descritor 1 para /dev/null. Qualquer código em C carregado no processo do host (a biblioteca do
// PC/SC, do diagnóstico) que escreva na saída padrão escreve no vazio, e não corrompe o quadro de
// native messaging; trocar só o `os.Stdout` do Go não protege do que o C escreve direto no
// descritor. A cópia leva CLOEXEC: os filhos dos módulos não a herdam.
func separarCanal() (*os.File, error) {
	syscall.ForkLock.RLock()
	fd, err := syscall.Dup(1)
	if err == nil {
		syscall.CloseOnExec(fd)
	}
	syscall.ForkLock.RUnlock()
	if err != nil {
		return nil, err
	}
	nulo, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	defer nulo.Close()
	if err := duplicarPara(int(nulo.Fd()), 1); err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	return os.NewFile(uintptr(fd), "canal"), nil
}
