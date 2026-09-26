/*
 * pcsc-lite FALSO, para os testes: as quatro funções que o programa usa, com leitoras de mentira.
 * O comportamento vem da variável ASSINADOR_PCSC_FALSO, lida só por este arquivo:
 *
 *   (ausente) ou "cartao": duas leitoras, a primeira com cartão (ATR 3B8F8001), a segunda vazia;
 *   "cresce": a lista de leitoras cresce entre o pedido do tamanho e o da lista (a primeira
 *     leitura devolve SCARD_E_INSUFFICIENT_BUFFER), como quando se conecta uma leitora no meio;
 *   "atr-grande": o cartão declara um ATR acima de 33 bytes;
 *   "mudo": o cartão está na leitora e não responde;
 *   "enorme": a lista de leitoras passa do teto;
 *   "escreve": escreve na saída padrão (o descritor 1) ao abrir o contexto, como uma biblioteca
 *     descuidada, para provar que isso não corrompe o canal do host.
 *
 * Só os testes o compilam (gcc -shared -fPIC -I nativo/internal/pcsc). Não vai para release.
 */
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include "tipos.h"

#define SUCESSO 0L
#define BUFFER_PEQUENO ((assinador_long)0x80100008L)
#define PRESENTE 0x0020UL
#define VAZIO 0x0010UL
#define MUDO 0x0200UL

static const char lista_curta[] = "Leitora Falsa A 00 00\0";
static const char lista_longa[] = "Leitora Falsa A 00 00\0Leitora Falsa B 01 00\0";
static const unsigned char atr[] = {0x3B, 0x8F, 0x80, 0x01};
static int leituras;

static int modo(const char *nome) {
	const char *m = getenv("ASSINADOR_PCSC_FALSO");
	if (m == NULL || *m == '\0') return strcmp(nome, "cartao") == 0;
	return strcmp(m, nome) == 0;
}

assinador_long SCardEstablishContext(assinador_dword escopo, const void *r1, const void *r2, assinador_contexto *ctx) {
	(void)escopo;
	(void)r1;
	(void)r2;
	leituras = 0;
	if (modo("escreve")) {
		ssize_t n = write(1, "lixo que a biblioteca escreveu na saida padrao\n", 47);
		(void)n;
	}
	*ctx = 42;
	return SUCESSO;
}

assinador_long SCardReleaseContext(assinador_contexto ctx) {
	(void)ctx;
	return SUCESSO;
}

assinador_long SCardListReaders(assinador_contexto ctx, const char *grupos, char *nomes, assinador_dword *tamanho) {
	(void)ctx;
	(void)grupos;
	/* A lista, com o NUL que a encerra. */
	const char *lista = lista_longa;
	assinador_dword tam = sizeof(lista_longa);
	if (modo("enorme")) {
		*tamanho = 1000000;
		return SUCESSO;
	}
	if (modo("cresce") && leituras < 2) {
		/* Primeiro pedido de tamanho: a lista curta. Na leitura, ela já cresceu. */
		if (nomes == NULL) {
			leituras++;
			*tamanho = sizeof(lista_curta);
			return SUCESSO;
		}
		leituras++;
		*tamanho = sizeof(lista_longa);
		return BUFFER_PEQUENO;
	}
	if (nomes == NULL) {
		*tamanho = tam;
		return SUCESSO;
	}
	if (*tamanho < tam) {
		*tamanho = tam;
		return BUFFER_PEQUENO;
	}
	memcpy(nomes, lista, tam);
	*tamanho = tam;
	return SUCESSO;
}

assinador_long SCardGetStatusChange(assinador_contexto ctx, assinador_dword prazo, assinador_estado_da_leitora *estados, assinador_dword n) {
	(void)ctx;
	(void)prazo;
	for (assinador_dword i = 0; i < n; i++) {
		if (strcmp(estados[i].szReader, "Leitora Falsa A 00 00") == 0) {
			estados[i].dwEventState = PRESENTE | (modo("mudo") ? MUDO : 0);
			memcpy(estados[i].rgbAtr, atr, sizeof(atr));
			estados[i].cbAtr = modo("atr-grande") ? ASSINADOR_MAX_ATR + 7 : sizeof(atr);
		} else {
			estados[i].dwEventState = VAZIO;
			estados[i].cbAtr = 0;
		}
	}
	return SUCESSO;
}
