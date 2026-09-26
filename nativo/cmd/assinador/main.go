// O Go recusa, desde a 1.23, certificado com número de série negativo, e há AC que já os emitiu.
// O programa não valida cadeia (quem valida é o servidor): ele só lê o certificado para listar e
// para conferir a assinatura que o cartão devolveu, e um certificado legítimo não pode sumir da
// lista por isso.

//go:debug x509negativeserial=1

// Comando assinador: o programa nativo do Assinador.
//
//	assinador <argumentos do navegador>    modo host: native messaging com a extensão (o padrão)
//	assinador modulo --caminho <módulo>    filho de um módulo PKCS#11 (só o próprio programa o lança)
//	assinador diagnostico [--json]         o diagnóstico no terminal, para o suporte (sem CPF)
//	assinador versao                       imprime a versão
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/bilhete"
	"github.com/Gottbrok/assinador/nativo/internal/catalogo"
	"github.com/Gottbrok/assinador/nativo/internal/diagnostico"
	"github.com/Gottbrok/assinador/nativo/internal/host"
	"github.com/Gottbrok/assinador/nativo/internal/origem"
	"github.com/Gottbrok/assinador/nativo/internal/pcsc"
)

// versao é trocada no build (`-ldflags "-X main.versao=1.0.0"`). Sempre `X.Y.Z`: é a forma que a
// biblioteca compara com a versão mínima, e qualquer outra a faz declarar o programa desatualizado.
// Em desenvolvimento, é a versão da PRÓXIMA publicação (o sufixo `~dev.N` fica só no pacote).
var versao = "1.0.0"

func main() {
	args := os.Args[1:]
	if len(args) == 3 && args[0] == "modulo" && args[1] == "--caminho" {
		os.Exit(executarModulo(args[2]))
	}
	if len(args) == 1 && args[0] == "versao" {
		fmt.Println(versao)
		return
	}
	if len(args) >= 1 && args[0] == "diagnostico" {
		os.Exit(executarDiagnostico(args[1:]))
	}
	os.Exit(executarHost(args))
}

// executarDiagnostico imprime no terminal o mesmo relatório que a operação `diagnostico` devolve à
// extensão: em frases, ou em JSON com `--json`.
func executarDiagnostico(args []string) int {
	emJSON := len(args) == 1 && args[0] == "--json"
	if len(args) > 1 || (len(args) == 1 && !emJSON) {
		fmt.Fprintln(os.Stderr, "uso: assinador diagnostico [--json]")
		return 2
	}
	executavel, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "assinador: sem o caminho do próprio programa:", err)
		return 1
	}
	relatorio, texto := diagnostico.Coletar(context.Background(), diagnostico.Fontes{
		Provedores:          provedores(executavel, 0),
		Leitoras:            pcsc.Consultar,
		ServicoDePropagacao: servicoDePropagacao,
		Agora:               time.Now,
		Versao:              versao,
		Plataforma:          plataforma(),
		Sistema:             diagnostico.SistemaOperacional(),
		ATRs:                catalogo.ATRs,
		Modulos:             catalogo.Modulos,
	})
	if emJSON {
		saida, _ := json.MarshalIndent(relatorio, "", "  ")
		fmt.Println(string(saida))
		return 0
	}
	fmt.Println(texto)
	return 0
}

func plataforma() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func executarHost(args []string) int {
	// O canal com a extensão é a saída padrão. Qualquer escrita perdida nela (do Go ou de código em
	// C, como a biblioteca do PC/SC) corromperia o quadro: o descritor 1 passa a ser /dev/null, e só
	// o host escreve na cópia do descritor verdadeiro.
	canal, err := separarCanal()
	if err != nil {
		fmt.Fprintln(os.Stderr, "assinador: não separei o canal da saída padrão:", err)
		return 1
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
		// No Windows, a janela que o Chrome informa é a mãe do diálogo de PIN do provedor.
		Provedores:          provedores(executavel, chamador.JanelaMae),
		Chaves:              chaves,
		Agora:               time.Now,
		Versao:              versao,
		Plataforma:          plataforma(),
		Leitoras:            pcsc.Consultar,
		ServicoDePropagacao: servicoDePropagacao,
		Sistema:             diagnostico.SistemaOperacional(),
	}
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()
	if err := h.Executar(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "assinador:", err)
		return 1
	}
	return 0
}
