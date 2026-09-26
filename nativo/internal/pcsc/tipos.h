/*
 * Os tipos do pcsc-lite FORA do macOS (o wintypes.h dele): DWORD é unsigned long e LONG é long.
 * Escritos aqui, e não incluídos do sistema, para o programa compilar sem o pacote de
 * desenvolvimento e abrir a biblioteca com dlopen, só quando o diagnóstico precisa dela. O teste
 * com a tag pcsc_cabecalho compara cada tamanho e cada deslocamento com os cabeçalhos de verdade.
 */
#ifndef ASSINADOR_PCSC_TIPOS_H
#define ASSINADOR_PCSC_TIPOS_H

typedef unsigned long assinador_dword;
typedef long assinador_long;
typedef assinador_long assinador_contexto;

#define ASSINADOR_MAX_ATR 33
#define ASSINADOR_ESCOPO_DO_SISTEMA 2UL

typedef struct {
	const char *szReader;
	void *pvUserData;
	assinador_dword dwCurrentState;
	assinador_dword dwEventState;
	assinador_dword cbAtr;
	unsigned char rgbAtr[ASSINADOR_MAX_ATR];
} assinador_estado_da_leitora;

#endif
