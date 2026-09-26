//go:build linux && pcsc_cabecalho

package pcsc

/*
#include <stddef.h>
#include <PCSC/winscard.h>
#include "tipos.h"

static size_t real_tam_dword(void) { return sizeof(DWORD); }
static size_t real_tam_long(void) { return sizeof(LONG); }
static size_t real_tam_contexto(void) { return sizeof(SCARDCONTEXT); }
static size_t real_tam_estado(void) { return sizeof(SCARD_READERSTATE); }
static size_t real_des_leitora(void) { return offsetof(SCARD_READERSTATE, szReader); }
static size_t real_des_atual(void) { return offsetof(SCARD_READERSTATE, dwCurrentState); }
static size_t real_des_evento(void) { return offsetof(SCARD_READERSTATE, dwEventState); }
static size_t real_des_tam_atr(void) { return offsetof(SCARD_READERSTATE, cbAtr); }
static size_t real_des_atr(void) { return offsetof(SCARD_READERSTATE, rgbAtr); }
static size_t real_max_atr(void) { return MAX_ATR_SIZE; }
static unsigned long real_escopo(void) { return SCARD_SCOPE_SYSTEM; }
static unsigned long real_sem_servico(void) { return (unsigned long)(unsigned int)SCARD_E_NO_SERVICE; }
static unsigned long real_servico_parou(void) { return (unsigned long)(unsigned int)SCARD_E_SERVICE_STOPPED; }
static unsigned long real_sem_leitora(void) { return (unsigned long)(unsigned int)SCARD_E_NO_READERS_AVAILABLE; }
static unsigned long real_tempo(void) { return (unsigned long)(unsigned int)SCARD_E_TIMEOUT; }
static unsigned long real_buffer_pequeno(void) { return (unsigned long)(unsigned int)SCARD_E_INSUFFICIENT_BUFFER; }
static unsigned long real_presente(void) { return SCARD_STATE_PRESENT; }
static unsigned long real_mudo(void) { return SCARD_STATE_MUTE; }
static size_t real_max_nome(void) { return MAX_READERNAME; }

static size_t nosso_tam_dword(void) { return sizeof(assinador_dword); }
static size_t nosso_tam_long(void) { return sizeof(assinador_long); }
static size_t nosso_tam_contexto(void) { return sizeof(assinador_contexto); }
static size_t nosso_tam_estado(void) { return sizeof(assinador_estado_da_leitora); }
static size_t nosso_des_leitora(void) { return offsetof(assinador_estado_da_leitora, szReader); }
static size_t nosso_des_atual(void) { return offsetof(assinador_estado_da_leitora, dwCurrentState); }
static size_t nosso_des_evento(void) { return offsetof(assinador_estado_da_leitora, dwEventState); }
static size_t nosso_des_tam_atr(void) { return offsetof(assinador_estado_da_leitora, cbAtr); }
static size_t nosso_des_atr(void) { return offsetof(assinador_estado_da_leitora, rgbAtr); }
static size_t nosso_max_atr(void) { return ASSINADOR_MAX_ATR; }
static unsigned long nosso_escopo(void) { return ASSINADOR_ESCOPO_DO_SISTEMA; }
*/
import "C"

// comparacaoComOCabecalho devolve, para cada medida, o valor do cabeçalho do pcsc-lite e o nosso.
func comparacaoComOCabecalho() map[string][2]uint64 {
	return map[string][2]uint64{
		"sizeof(DWORD)":                {uint64(C.real_tam_dword()), uint64(C.nosso_tam_dword())},
		"sizeof(LONG)":                 {uint64(C.real_tam_long()), uint64(C.nosso_tam_long())},
		"sizeof(SCARDCONTEXT)":         {uint64(C.real_tam_contexto()), uint64(C.nosso_tam_contexto())},
		"sizeof(SCARD_READERSTATE)":    {uint64(C.real_tam_estado()), uint64(C.nosso_tam_estado())},
		"offsetof(szReader)":           {uint64(C.real_des_leitora()), uint64(C.nosso_des_leitora())},
		"offsetof(dwCurrentState)":     {uint64(C.real_des_atual()), uint64(C.nosso_des_atual())},
		"offsetof(dwEventState)":       {uint64(C.real_des_evento()), uint64(C.nosso_des_evento())},
		"offsetof(cbAtr)":              {uint64(C.real_des_tam_atr()), uint64(C.nosso_des_tam_atr())},
		"offsetof(rgbAtr)":             {uint64(C.real_des_atr()), uint64(C.nosso_des_atr())},
		"MAX_ATR_SIZE":                 {uint64(C.real_max_atr()), uint64(C.nosso_max_atr())},
		"SCARD_SCOPE_SYSTEM":           {uint64(C.real_escopo()), uint64(C.nosso_escopo())},
		"SCARD_E_NO_SERVICE":           {uint64(C.real_sem_servico()), codigoSemServico},
		"SCARD_E_SERVICE_STOPPED":      {uint64(C.real_servico_parou()), codigoServicoParou},
		"SCARD_E_NO_READERS_AVAILABLE": {uint64(C.real_sem_leitora()), codigoSemLeitora},
		"SCARD_E_TIMEOUT":              {uint64(C.real_tempo()), codigoTempoEsgotado},
		"SCARD_E_INSUFFICIENT_BUFFER":  {uint64(C.real_buffer_pequeno()), codigoBufferPequeno},
		"SCARD_STATE_PRESENT":          {uint64(C.real_presente()), estadoPresente},
		"SCARD_STATE_MUTE":             {uint64(C.real_mudo()), estadoMudo},
		"MAX_READERNAME":               {uint64(C.real_max_nome()), tamanhoMaximoDoNome},
	}
}
