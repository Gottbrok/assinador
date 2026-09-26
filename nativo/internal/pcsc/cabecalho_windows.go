//go:build windows && pcsc_cabecalho

package pcsc

/*
#include <stddef.h>
#include <windows.h>
#include <winscard.h>

static size_t real_tam_estado(void) { return sizeof(SCARD_READERSTATEW); }
static size_t real_des_leitora(void) { return offsetof(SCARD_READERSTATEW, szReader); }
static size_t real_des_usuario(void) { return offsetof(SCARD_READERSTATEW, pvUserData); }
static size_t real_des_atual(void) { return offsetof(SCARD_READERSTATEW, dwCurrentState); }
static size_t real_des_evento(void) { return offsetof(SCARD_READERSTATEW, dwEventState); }
static size_t real_des_tam_atr(void) { return offsetof(SCARD_READERSTATEW, cbAtr); }
static size_t real_des_atr(void) { return offsetof(SCARD_READERSTATEW, rgbAtr); }
static size_t real_tam_atr(void) { return sizeof(((SCARD_READERSTATEW *)0)->rgbAtr); }
static unsigned long real_atr_da_norma(void) { return SCARD_ATR_LENGTH; }
static unsigned long real_escopo(void) { return SCARD_SCOPE_USER; }
static unsigned long real_sem_servico(void) { return (unsigned long)SCARD_E_NO_SERVICE; }
static unsigned long real_servico_parou(void) { return (unsigned long)SCARD_E_SERVICE_STOPPED; }
static unsigned long real_sem_leitora(void) { return (unsigned long)SCARD_E_NO_READERS_AVAILABLE; }
static unsigned long real_tempo(void) { return (unsigned long)SCARD_E_TIMEOUT; }
static unsigned long real_buffer_pequeno(void) { return (unsigned long)SCARD_E_INSUFFICIENT_BUFFER; }
static unsigned long real_presente(void) { return SCARD_STATE_PRESENT; }
static unsigned long real_mudo(void) { return SCARD_STATE_MUTE; }
*/
import "C"

import "unsafe"

// comparacaoComOCabecalho devolve, para cada medida, o valor do `winscard.h` e o nosso.
func comparacaoComOCabecalho() map[string][2]uint64 {
	var e estadoDaLeitora
	return map[string][2]uint64{
		"sizeof(SCARD_READERSTATEW)":   {uint64(C.real_tam_estado()), uint64(unsafe.Sizeof(e))},
		"offsetof(szReader)":           {uint64(C.real_des_leitora()), uint64(unsafe.Offsetof(e.leitora))},
		"offsetof(pvUserData)":         {uint64(C.real_des_usuario()), uint64(unsafe.Offsetof(e.usuario))},
		"offsetof(dwCurrentState)":     {uint64(C.real_des_atual()), uint64(unsafe.Offsetof(e.atual))},
		"offsetof(dwEventState)":       {uint64(C.real_des_evento()), uint64(unsafe.Offsetof(e.evento))},
		"offsetof(cbAtr)":              {uint64(C.real_des_tam_atr()), uint64(unsafe.Offsetof(e.tamanhoATR))},
		"offsetof(rgbAtr)":             {uint64(C.real_des_atr()), uint64(unsafe.Offsetof(e.atr))},
		"sizeof(rgbAtr)":               {uint64(C.real_tam_atr()), uint64(len(e.atr))},
		"SCARD_ATR_LENGTH":             {uint64(C.real_atr_da_norma()), atrMaximoPelaNorma},
		"SCARD_SCOPE_USER":             {uint64(C.real_escopo()), escopoDoUsuario},
		"SCARD_E_NO_SERVICE":           {uint64(C.real_sem_servico()), codigoSemServico},
		"SCARD_E_SERVICE_STOPPED":      {uint64(C.real_servico_parou()), codigoServicoParou},
		"SCARD_E_NO_READERS_AVAILABLE": {uint64(C.real_sem_leitora()), codigoSemLeitora},
		"SCARD_E_TIMEOUT":              {uint64(C.real_tempo()), codigoTempoEsgotado},
		"SCARD_E_INSUFFICIENT_BUFFER":  {uint64(C.real_buffer_pequeno()), codigoBufferPequeno},
		"SCARD_STATE_PRESENT":          {uint64(C.real_presente()), estadoPresente},
		"SCARD_STATE_MUTE":             {uint64(C.real_mudo()), estadoMudo},
	}
}
