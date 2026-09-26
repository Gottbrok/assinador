//go:build linux

package main

import "syscall"

// duplicarPara é o dup2; no Linux, o Dup3 existe em toda arquitetura (o Dup2 falta no arm64).
func duplicarPara(velho, novo int) error {
	return syscall.Dup3(velho, novo, 0)
}
