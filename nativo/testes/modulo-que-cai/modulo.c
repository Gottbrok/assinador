/*
 * Módulo PKCS#11 de TESTE que falha de propósito, para provar o isolamento do filho (regra 8 do
 * CLAUDE.md: biblioteca de fabricante que derruba o processo derruba só o filho). O comportamento
 * vem da variável ASSINADOR_MODULO_DE_TESTE, lida só por este arquivo:
 *
 *   (ausente) ou "cai": aborta no C_Initialize, como uma biblioteca com defeito;
 *   "trava": dorme para sempre no C_Initialize, como uma biblioteca esperando um cartão.
 *
 * Só os testes o compilam (gcc -shared -fPIC). Não vai para release.
 */
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

typedef unsigned long CK_RV;

typedef struct {
	unsigned char major;
	unsigned char minor;
} CK_VERSION;

/* A lista de funções tem 68 ponteiros na 2.40; só o primeiro (C_Initialize) é usado. */
typedef struct {
	CK_VERSION version;
	void *funcoes[68];
} lista_de_funcoes;

static lista_de_funcoes lista;

static CK_RV inicializar(void *argumentos) {
	const char *modo = getenv("ASSINADOR_MODULO_DE_TESTE");
	(void)argumentos;
	if (modo != NULL && strcmp(modo, "trava") == 0) {
		for (;;) sleep(60);
	}
	abort();
	return 0;
}

CK_RV C_GetFunctionList(lista_de_funcoes **destino) {
	lista.version.major = 2;
	lista.version.minor = 40;
	lista.funcoes[0] = (void *)inicializar;
	*destino = &lista;
	return 0;
}
