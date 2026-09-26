//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"unsafe"

	win "golang.org/x/sys/windows"
)

// descritorDoCRTEhNUL diz se o descritor 1 de um C runtime aponta para um dispositivo de caractere
// (o NUL), e não para o pipe do navegador.
func descritorDoCRTEhNUL(modulo win.Handle) bool {
	handleDoDescritor, err := win.GetProcAddress(modulo, "_get_osfhandle")
	if err != nil {
		return false
	}
	h, _, _ := syscall.SyscallN(handleDoDescritor, 1)
	tipo, err := win.GetFileType(win.Handle(h))
	return err == nil && tipo == win.FILE_TYPE_CHAR
}

// escreverPeloCRT escreve pelo descritor 1 de um C runtime, como faria o `printf` de uma DLL de
// fabricante: carrega o CRT se ainda não estiver (e aí ele já nasce apontando para a saída do
// processo, que é NUL) e chama o `_write` dele.
func escreverPeloCRT(dll, texto string) bool {
	modulo, err := win.LoadLibraryEx(dll, 0, win.LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		return false
	}
	escrever, err := win.GetProcAddress(modulo, "_write")
	if err != nil {
		return false
	}
	b := []byte(texto)
	_, _, _ = syscall.SyscallN(escrever, 1, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return true
}

// O binário de teste faz o papel do host no teste do canal: separa o canal, escreve "perdida" pelo
// os.Stdout do Go, pelo handle da saída padrão do processo e pelo descritor 1 dos dois C runtimes
// compartilhados, e "canal" pela cópia. Os dois C runtimes são carregados ANTES de separar o canal:
// é o caso que o `_dup2` resolve (o que carrega depois já nasce em NUL e não provaria nada).
func TestMain(m *testing.M) {
	if os.Getenv("ASSINADOR_TESTE_DO_CANAL") == "1" {
		var crts []win.Handle
		for _, dll := range []string{"msvcrt.dll", "ucrtbase.dll"} {
			// O ucrtbase existe do Windows 10 em diante; o msvcrt, sempre.
			if modulo, err := win.LoadLibraryEx(dll, 0, win.LOAD_LIBRARY_SEARCH_SYSTEM32); err == nil {
				crts = append(crts, modulo)
			} else if dll == "msvcrt.dll" {
				os.Exit(5)
			}
		}
		original := os.Stdout.Fd()
		canal, err := separarCanal()
		if err != nil {
			os.Exit(3)
		}
		for _, modulo := range crts {
			if !descritorDoCRTEhNUL(modulo) {
				os.Exit(7)
			}
		}
		informacao := win.NewLazySystemDLL("kernel32.dll").NewProc("GetHandleInformation")
		// O handle original está protegido contra fechamento: os `_dup2` dos C runtimes o fecham sem
		// efeito, e o número dele nunca volta ao Windows para ser dado a outro recurso.
		var flagsDoOriginal uint32
		if r, _, _ := informacao.Call(original, uintptr(unsafe.Pointer(&flagsDoOriginal))); r == 0 || flagsDoOriginal&handleProtegidoContraFechamento == 0 {
			os.Exit(6)
		}
		_, _ = os.Stdout.WriteString("perdida-pelo-go ")
		if h, err := win.GetStdHandle(win.STD_OUTPUT_HANDLE); err == nil {
			b := []byte("perdida-pelo-handle ")
			var n uint32
			_ = win.WriteFile(h, b, &n, nil)
		}
		if !escreverPeloCRT("msvcrt.dll", "perdida-pelo-msvcrt ") {
			os.Exit(5)
		}
		// O ucrtbase existe do Windows 10 em diante; sem ele, não há o que desviar.
		_ = escreverPeloCRT("ucrtbase.dll", "perdida-pelo-ucrt ")
		var flags uint32
		r, _, _ := informacao.Call(canal.Fd(), uintptr(unsafe.Pointer(&flags)))
		if r == 0 || flags&win.HANDLE_FLAG_INHERIT != 0 {
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
		t.Fatalf("o processo de teste falhou (3: não separou; 4: a cópia herdável; 5: sem o msvcrt; 6: o original sem proteção; 7: o descritor 1 de um C runtime já carregado não foi para NUL): %v", err)
	}
	if string(saida) != "canal" {
		t.Fatalf("a saída verdadeira recebeu %q: o que foi escrito na saída padrão vazou para o canal", saida)
	}
}
