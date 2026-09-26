//go:build darwin

package main

import "syscall"

// duplicarPara é o dup2.
func duplicarPara(velho, novo int) error {
	return syscall.Dup2(velho, novo)
}
