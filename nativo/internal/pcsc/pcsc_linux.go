//go:build linux

package pcsc

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>
#include "tipos.h"

typedef assinador_long (*assinador_fn_estabelecer)(assinador_dword, const void *, const void *, assinador_contexto *);
typedef assinador_long (*assinador_fn_listar)(assinador_contexto, const char *, char *, assinador_dword *);
typedef assinador_long (*assinador_fn_estado)(assinador_contexto, assinador_dword, assinador_estado_da_leitora *, assinador_dword);
typedef assinador_long (*assinador_fn_liberar)(assinador_contexto);

typedef struct {
	void *h;
	assinador_fn_estabelecer estabelecer;
	assinador_fn_listar listar;
	assinador_fn_estado estado;
	assinador_fn_liberar liberar;
} assinador_pcsc;

static int assinador_pcsc_abrir(assinador_pcsc *p, const char *biblioteca) {
	memset(p, 0, sizeof(*p));
	p->h = dlopen(biblioteca, RTLD_NOW | RTLD_LOCAL);
	if (p->h == NULL) return 0;
	p->estabelecer = (assinador_fn_estabelecer) dlsym(p->h, "SCardEstablishContext");
	p->listar = (assinador_fn_listar) dlsym(p->h, "SCardListReaders");
	p->estado = (assinador_fn_estado) dlsym(p->h, "SCardGetStatusChange");
	p->liberar = (assinador_fn_liberar) dlsym(p->h, "SCardReleaseContext");
	if (!p->estabelecer || !p->listar || !p->estado || !p->liberar) {
		dlclose(p->h);
		p->h = NULL;
		return 0;
	}
	return 1;
}

static void assinador_pcsc_fechar(assinador_pcsc *p) {
	if (p->h != NULL) dlclose(p->h);
	p->h = NULL;
}

static assinador_long assinador_pcsc_estabelecer(assinador_pcsc *p, assinador_contexto *ctx) {
	return p->estabelecer(ASSINADOR_ESCOPO_DO_SISTEMA, NULL, NULL, ctx);
}

static assinador_long assinador_pcsc_listar(assinador_pcsc *p, assinador_contexto ctx, char *nomes, assinador_dword *tamanho) {
	return p->listar(ctx, NULL, nomes, tamanho);
}

// Tempo zero e estado de partida "desconhecido": a chamada volta na hora com o estado atual de cada
// leitora e o ATR do cartão, sem conectar a ele.
static assinador_long assinador_pcsc_estado(assinador_pcsc *p, assinador_contexto ctx, assinador_estado_da_leitora *e, assinador_dword n) {
	return p->estado(ctx, 0, e, n);
}

static void assinador_pcsc_liberar(assinador_pcsc *p, assinador_contexto ctx) {
	p->liberar(ctx);
}
*/
import "C"

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unsafe"
)

// biblioteca é o pcsc-lite. Variável para o teste da biblioteca ausente.
var biblioteca = "libpcsclite.so.1"

func falha(codigo C.assinador_long) Resultado {
	c := uint32(codigo)
	return Resultado{Estado: estadoDoCodigo(c), Detalhe: fmt.Sprintf("0x%08X", c), Leitoras: []Leitora{}}
}

// tentativasDeListar: a lista pode mudar entre o pedido do tamanho e o da lista (uma leitora
// conectada no meio), e aí o PC/SC responde que o espaço não bastou.
const tentativasDeListar = 3

// listarNomes pede o tamanho da lista, aloca e lê; se a lista cresceu no meio
// (SCARD_E_INSUFFICIENT_BUFFER), tenta de novo. Lista acima do teto é falha, e não "nenhuma
// leitora". O que o PC/SC diz que escreveu nunca é lido além do que foi alocado.
func listarNomes(p *C.assinador_pcsc, ctx C.assinador_contexto) ([]string, *Resultado) {
	for i := 0; i < tentativasDeListar; i++ {
		var tamanho C.assinador_dword
		if rv := C.assinador_pcsc_listar(p, ctx, nil, &tamanho); rv != 0 {
			r := falha(rv)
			return nil, &r
		}
		if tamanho == 0 {
			return nil, nil
		}
		if tamanho > tamanhoMaximoDaLista {
			return nil, &Resultado{Estado: EstadoFalhou, Detalhe: "lista de leitoras acima do teto", Leitoras: []Leitora{}}
		}
		alocado := tamanho
		buf := (*C.char)(C.calloc(C.size_t(alocado)+1, 1))
		rv := C.assinador_pcsc_listar(p, ctx, buf, &tamanho)
		if uint32(rv) == codigoBufferPequeno {
			C.free(unsafe.Pointer(buf))
			continue
		}
		if rv != 0 {
			C.free(unsafe.Pointer(buf))
			r := falha(rv)
			return nil, &r
		}
		lido := min(tamanho, alocado)
		nomes := nomesDaLista(C.GoBytes(unsafe.Pointer(buf), C.int(lido)))
		C.free(unsafe.Pointer(buf))
		return nomes, nil
	}
	return nil, &Resultado{Estado: EstadoFalhou, Detalhe: "a lista de leitoras não parou de mudar", Leitoras: []Leitora{}}
}

// Consultar lê as leitoras e o estado de cada uma. Pode bloquear enquanto o `pcscd` responde: quem
// chama põe prazo.
func Consultar() Resultado {
	cBiblioteca := C.CString(biblioteca)
	defer C.free(unsafe.Pointer(cBiblioteca))
	var p C.assinador_pcsc
	if C.assinador_pcsc_abrir(&p, cBiblioteca) == 0 {
		return Resultado{Estado: EstadoSemBiblioteca, Leitoras: []Leitora{}}
	}
	defer C.assinador_pcsc_fechar(&p)

	var ctx C.assinador_contexto
	if rv := C.assinador_pcsc_estabelecer(&p, &ctx); rv != 0 {
		return falha(rv)
	}
	defer C.assinador_pcsc_liberar(&p, ctx)

	nomes, recusa := listarNomes(&p, ctx)
	if recusa != nil {
		return *recusa
	}
	if len(nomes) == 0 {
		return Resultado{Estado: EstadoSemLeitora, Leitoras: []Leitora{}}
	}

	estados := (*[maximoDeLeitoras]C.assinador_estado_da_leitora)(C.calloc(C.size_t(len(nomes)), C.size_t(unsafe.Sizeof(C.assinador_estado_da_leitora{}))))
	defer C.free(unsafe.Pointer(estados))
	cNomes := make([]*C.char, len(nomes))
	for i, nome := range nomes {
		cNomes[i] = C.CString(nome)
		estados[i].szReader = cNomes[i]
	}
	defer func() {
		for _, c := range cNomes {
			C.free(unsafe.Pointer(c))
		}
	}()
	if rv := C.assinador_pcsc_estado(&p, ctx, &estados[0], C.assinador_dword(len(nomes))); rv != 0 && uint32(rv) != codigoTempoEsgotado {
		return falha(rv)
	}
	saida := Resultado{Estado: EstadoOk, Leitoras: make([]Leitora, 0, len(nomes))}
	for i, nome := range nomes {
		e := estados[i]
		l := Leitora{Nome: nome, ComCartao: e.dwEventState&estadoPresente != 0, Mudo: e.dwEventState&estadoMudo != 0}
		if l.ComCartao && e.cbAtr > 0 && e.cbAtr <= C.ASSINADOR_MAX_ATR {
			atr := C.GoBytes(unsafe.Pointer(&e.rgbAtr[0]), C.int(e.cbAtr))
			l.ATR = strings.ToUpper(hex.EncodeToString(atr))
		}
		saida.Leitoras = append(saida.Leitoras, l)
	}
	return saida
}
