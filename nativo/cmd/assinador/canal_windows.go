//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"

	win "golang.org/x/sys/windows"
)

// separarCanal guarda uma CÓPIA do handle da saída padrão para o canal com a extensão e aponta a
// saída padrão do processo para NUL. No Windows o provedor do fabricante (CSP ou KSP) é uma DLL que
// roda DENTRO do host, e um `printf` dela corromperia o quadro de native messaging (regra 12; a F6a
// decidiu que o canal precisa da mesma separação do Linux).
//
// São três camadas: o `os.Stdout` do Go; o handle do processo (`SetStdHandle`), que o C runtime de
// toda DLL carregada DEPOIS lê ao iniciar; e o descritor 1 dos C runtimes compartilhados que JÁ
// estavam carregados (o `msvcrt.dll` e o `ucrtbase.dll`, que DLLs do sistema trazem cedo), trocado
// por `_dup2`, como o `dup2` do Linux. A cópia não é herdável: o host não lança filho no Windows.
//
// O handle ORIGINAL fica protegido contra fechamento antes dos `_dup2`: cada C runtime o guarda
// como descritor 1 e o fecha ao trocar, e com os dois carregados o segundo fecharia de novo um
// número que o Windows já pode ter dado a outro recurso (uma thread do runtime do Go, por exemplo).
// Protegido, o fechamento falha sem estrago (o `_dup2` do UCRT e o do `msvcrt` ignoram essa falha,
// e seguem), e o número nunca volta ao Windows.
func separarCanal() (*os.File, error) {
	processo := win.CurrentProcess()
	original := win.Handle(os.Stdout.Fd())
	var copia win.Handle
	if err := win.DuplicateHandle(processo, original, processo, &copia, 0, false, win.DUPLICATE_SAME_ACCESS); err != nil {
		return nil, err
	}
	if err := win.SetHandleInformation(original, handleProtegidoContraFechamento, handleProtegidoContraFechamento); err != nil {
		_ = win.CloseHandle(copia)
		return nil, err
	}
	nome, _ := win.UTF16PtrFromString("NUL")
	nulo, err := win.CreateFile(nome, win.GENERIC_WRITE, win.FILE_SHARE_READ|win.FILE_SHARE_WRITE, nil, win.OPEN_EXISTING, 0, 0)
	if err != nil {
		_ = win.CloseHandle(copia)
		return nil, err
	}
	if err := win.SetStdHandle(win.STD_OUTPUT_HANDLE, nulo); err != nil {
		_ = win.CloseHandle(copia)
		_ = win.CloseHandle(nulo)
		return nil, err
	}
	// O `os.Stdout` antigo também fica vivo para sempre, para o finalizador dele nem tentar fechar o
	// handle original.
	saidasAntigas = append(saidasAntigas, os.Stdout)
	os.Stdout = os.NewFile(uintptr(nulo), "NUL")
	for _, crt := range []string{"msvcrt.dll", "ucrtbase.dll"} {
		if err := desviarDescritorDoCRT(crt); err != nil {
			_ = win.CloseHandle(copia)
			return nil, err
		}
	}
	return os.NewFile(uintptr(copia), "canal"), nil
}

// handleProtegidoContraFechamento é o HANDLE_FLAG_PROTECT_FROM_CLOSE (winbase.h), que o
// `golang.org/x/sys/windows` não declara.
const handleProtegidoContraFechamento = 0x00000002

// saidasAntigas guarda os `os.Stdout` substituídos, para o finalizador nunca rodar sobre eles.
var saidasAntigas []*os.File

// desviarDescritorDoCRT aponta o descritor 1 de um C runtime JÁ carregado para NUL. O que ainda não
// foi carregado não precisa: ao iniciar, ele lê a saída padrão do processo, que já é NUL.
func desviarDescritorDoCRT(dll string) error {
	nome, _ := win.UTF16PtrFromString(dll)
	var modulo win.Handle
	// Sem aumentar a contagem de referência: só se pergunta se ele está carregado.
	if err := win.GetModuleHandleEx(win.GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT, nome, &modulo); err != nil {
		return nil
	}
	abrir, err := win.GetProcAddress(modulo, "_open")
	if err != nil {
		return err
	}
	duplicar, err := win.GetProcAddress(modulo, "_dup2")
	if err != nil {
		return err
	}
	fechar, err := win.GetProcAddress(modulo, "_close")
	if err != nil {
		return err
	}
	caminho := []byte("NUL\x00")
	const somenteEscrita = 1 // _O_WRONLY
	fd, _, _ := syscall.SyscallN(abrir, uintptr(unsafe.Pointer(&caminho[0])), somenteEscrita)
	if int32(fd) < 0 {
		return syscall.EINVAL
	}
	r, _, _ := syscall.SyscallN(duplicar, fd, 1)
	_, _, _ = syscall.SyscallN(fechar, fd)
	if int32(r) != 0 {
		return syscall.EINVAL
	}
	return nil
}
