/*
 * Módulo PKCS#11 de TESTE que falha de propósito, para provar o isolamento do filho (regra 8 do
 * CLAUDE.md: biblioteca de fabricante que derruba o processo derruba só o filho). O comportamento
 * vem da variável ASSINADOR_MODULO_DE_TESTE, lida só por este arquivo:
 *
 *   (ausente) ou "cai": aborta no C_Initialize, como uma biblioteca com defeito;
 *   "trava": dorme para sempre no C_Initialize, como uma biblioteca esperando um cartão;
 *   "trava-no-fim": responde normalmente (zero slots) e dorme para sempre no C_Finalize, como uma
 *   biblioteca que trava ao encerrar DEPOIS de ter feito o trabalho.
 *
 * Só os testes o compilam (gcc -shared -fPIC). Não vai para release.
 */
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

typedef unsigned long CK_RV;
typedef unsigned long CK_ULONG;

#define CKR_OK 0x0UL
#define CKR_FUNCTION_FAILED 0x6UL

typedef struct {
	unsigned char major;
	unsigned char minor;
} CK_VERSION;

/* A lista de funções tem 68 ponteiros na 2.40. Os usados: 0 C_Initialize, 1 C_Finalize,
 * 2 C_GetInfo, 4 C_GetSlotList. */
typedef struct {
	CK_VERSION version;
	void *funcoes[68];
} lista_de_funcoes;

static lista_de_funcoes lista;

static int modo(const char *nome) {
	const char *m = getenv("ASSINADOR_MODULO_DE_TESTE");
	return m != NULL && strcmp(m, nome) == 0;
}

static void dormir_para_sempre(void) {
	for (;;) sleep(60);
}

static CK_RV inicializar(void *argumentos) {
	(void)argumentos;
	if (modo("trava-no-fim")) return CKR_OK;
	if (modo("trava")) dormir_para_sempre();
	abort();
	return CKR_OK;
}

static CK_RV finalizar(void *reservado) {
	(void)reservado;
	if (modo("trava-no-fim")) dormir_para_sempre();
	return CKR_OK;
}

static CK_RV informar(void *info) {
	(void)info;
	return CKR_FUNCTION_FAILED;
}

static CK_RV listar_slots(unsigned char com_token, CK_ULONG *slots, CK_ULONG *quantos) {
	(void)com_token;
	(void)slots;
	*quantos = 0;
	return CKR_OK;
}

CK_RV C_GetFunctionList(lista_de_funcoes **destino) {
	lista.version.major = 2;
	lista.version.minor = 40;
	lista.funcoes[0] = (void *)inicializar;
	lista.funcoes[1] = (void *)finalizar;
	lista.funcoes[2] = (void *)informar;
	lista.funcoes[4] = (void *)listar_slots;
	*destino = &lista;
	return CKR_OK;
}
