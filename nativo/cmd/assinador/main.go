// O Go recusa, desde a 1.23, certificado com número de série negativo, e há AC que já os emitiu.
// O programa não valida cadeia (quem valida é o servidor): ele só lê o certificado para listar e
// para conferir a assinatura que o cartão devolveu, e um certificado legítimo não pode sumir da
// lista por isso.

//go:debug x509negativeserial=1

// Comando assinador: o programa nativo do Assinador.
//
//	assinador <argumentos do navegador>    modo host: native messaging com a extensão (o padrão)
//	assinador modulo --caminho <módulo>    filho de um módulo PKCS#11 (só o próprio programa o lança)
//	assinador versao                       imprime a versão
//
// O modo `diagnostico` no terminal, para o suporte, entra na F2b.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/bilhete"
	"github.com/Gottbrok/assinador/nativo/internal/host"
	"github.com/Gottbrok/assinador/nativo/internal/origem"
)

// versao é trocada no build de release (`-ldflags "-X main.versao=1.0.0"`).
var versao = "0.1.0-dev"

func main() {
	args := os.Args[1:]
	if len(args) == 3 && args[0] == "modulo" && args[1] == "--caminho" {
		os.Exit(executarModulo(args[2]))
	}
	if len(args) == 1 && args[0] == "versao" {
		fmt.Println(versao)
		return
	}
	os.Exit(executarHost(args))
}

func executarHost(args []string) int {
	// O canal com a extensão é a saída padrão. Qualquer escrita perdida em `os.Stdout` (de uma
	// dependência, de um descuido) corromperia o quadro: ela passa a ir para /dev/null, e só o
	// host escreve no descritor verdadeiro.
	canal := os.Stdout
	if nulo, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		os.Stdout = nulo
	}
	if buildDeDesenvolvimento {
		fmt.Fprintln(os.Stderr, "assinador: build de DESENVOLVIMENTO (aceita chaves dev e localhost)")
	}
	chamador, ok := origem.LerChamador(args)
	chaves, avisos := bilhete.ChavesDoPrograma()
	for _, a := range avisos {
		fmt.Fprintln(os.Stderr, "assinador:", a)
	}
	executavel, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "assinador: sem o caminho do próprio programa:", err)
		return 1
	}
	h := &host.Host{
		Entrada:    os.Stdin,
		Saida:      canal,
		Chamador:   chamador,
		ChamadorOk: ok,
		Provedores: provedores(executavel),
		Chaves:     chaves,
		Agora:      time.Now,
		Versao:     versao,
		Plataforma: runtime.GOOS + "-" + runtime.GOARCH,
	}
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()
	if err := h.Executar(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "assinador:", err)
		return 1
	}
	return 0
}
