// Package pcsc lê as leitoras e o ATR dos cartões pelo PC/SC, para o diagnóstico. Só estado: nunca
// conecta ao cartão e nunca manda comando (APDU) a ele. No Linux a biblioteca (pcsc-lite) é aberta
// com dlopen, na hora da consulta: sem o `pcscd` instalado, o programa abre do mesmo jeito e o
// diagnóstico diz o que falta, em frase.
package pcsc

import "strings"

// Estados de uma consulta.
const (
	EstadoOk            = "ok"
	EstadoSemBiblioteca = "sem-biblioteca"
	EstadoSemServico    = "sem-servico"
	EstadoSemLeitora    = "sem-leitora"
	EstadoFalhou        = "falhou"
)

// Leitora é uma leitora e o cartão nela, se houver.
type Leitora struct {
	Nome      string `json:"nome"`
	ComCartao bool   `json:"comCartao"`
	// Mudo é o cartão presente que não responde (mal encaixado, ou com defeito).
	Mudo bool `json:"mudo,omitempty"`
	// ATR em hexadecimal maiúsculo, sem espaço; vazio sem cartão.
	ATR string `json:"atr,omitempty"`
}

// Resultado de uma consulta. `Detalhe` é para o suporte (o código do PC/SC), nunca para a pessoa.
type Resultado struct {
	Estado   string    `json:"estado"`
	Detalhe  string    `json:"detalhe,omitempty"`
	Leitoras []Leitora `json:"leitoras"`
}

// Códigos do PC/SC que o diagnóstico distingue (pcsclite.h).
const (
	codigoSemServico     = 0x8010001D // SCARD_E_NO_SERVICE
	codigoServicoParou   = 0x8010001E // SCARD_E_SERVICE_STOPPED
	codigoSemLeitora     = 0x8010002E // SCARD_E_NO_READERS_AVAILABLE
	codigoTempoEsgotado  = 0x8010000A // SCARD_E_TIMEOUT
	codigoBufferPequeno  = 0x80100008 // SCARD_E_INSUFFICIENT_BUFFER
	estadoPresente       = 0x0020     // SCARD_STATE_PRESENT
	estadoMudo           = 0x0200     // SCARD_STATE_MUTE
	maximoDeLeitoras     = 16
	tamanhoMaximoDoNome  = 128 // MAX_READERNAME
	tamanhoMaximoDaLista = maximoDeLeitoras * (tamanhoMaximoDoNome + 1)
)

// estadoDoCodigo traduz o código de erro de uma chamada ao PC/SC.
func estadoDoCodigo(codigo uint32) string {
	switch codigo {
	case codigoSemServico, codigoServicoParou:
		return EstadoSemServico
	case codigoSemLeitora:
		return EstadoSemLeitora
	}
	return EstadoFalhou
}

// nomesDaLista separa a lista de nomes do SCardListReaders (cada nome termina em NUL, e a lista
// termina num NUL a mais), com teto de leitoras e de tamanho.
func nomesDaLista(bruto []byte) []string {
	var nomes []string
	for _, nome := range strings.Split(string(bruto), "\x00") {
		if nome == "" {
			continue
		}
		if len(nome) > tamanhoMaximoDoNome || len(nomes) == maximoDeLeitoras {
			break
		}
		nomes = append(nomes, nome)
	}
	return nomes
}
