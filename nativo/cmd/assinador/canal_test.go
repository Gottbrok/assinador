//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
)

// O binário de teste faz o papel do host no teste do canal: separa o canal, escreve "perdida" pelo
// os.Stdout do Go e direto no descritor 1 (como faria uma biblioteca em C), e "canal" pela cópia.
func TestMain(m *testing.M) {
	if os.Getenv("ASSINADOR_TESTE_DO_CANAL") == "1" {
		canal, err := separarCanal()
		if err != nil {
			os.Exit(3)
		}
		_, _ = os.Stdout.WriteString("perdida-pelo-go ")
		_, _ = syscall.Write(1, []byte("perdida-pelo-descritor "))
		flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, canal.Fd(), syscall.F_GETFD, 0)
		if errno != 0 || flags&syscall.FD_CLOEXEC == 0 {
			os.Exit(4)
		}
		_, _ = canal.WriteString("canal")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestCanalSeparadoDaSaidaPadrao(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^$")
	cmd.Env = append(os.Environ(), "ASSINADOR_TESTE_DO_CANAL=1")
	saida, err := cmd.Output()
	if err != nil {
		t.Fatalf("o processo de teste falhou (3: não separou; 4: a cópia sem CLOEXEC): %v", err)
	}
	if string(saida) != "canal" {
		t.Fatalf("a saída verdadeira recebeu %q: o que foi escrito no descritor 1 vazou para o canal", saida)
	}
}
