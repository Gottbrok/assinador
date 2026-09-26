//go:build !windows

package pkcs11

/*
#cgo linux LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>

typedef unsigned long assinador_ck_ulong;
typedef unsigned char assinador_ck_byte;
typedef assinador_ck_ulong (*assinador_fn_login)(assinador_ck_ulong, assinador_ck_ulong, assinador_ck_byte *, assinador_ck_ulong);
typedef assinador_ck_ulong (*assinador_fn_obter_lista)(void **);

typedef struct {
	assinador_ck_byte major;
	assinador_ck_byte minor;
} assinador_ck_version;

// O começo do CK_FUNCTION_LIST do PKCS#11 (o da 2.x, que a 3.x mantém como prefixo): a versão e os
// 18 ponteiros ANTES de C_Login, na ordem do padrão (C_Initialize, C_Finalize, C_GetInfo,
// C_GetFunctionList, C_GetSlotList, C_GetSlotInfo, C_GetTokenInfo, C_GetMechanismList,
// C_GetMechanismInfo, C_InitToken, C_InitPIN, C_SetPIN, C_OpenSession, C_CloseSession,
// C_CloseAllSessions, C_GetSessionInfo, C_GetOperationState, C_SetOperationState). Fora do Windows
// a estrutura não é empacotada: a versão ocupa o alinhamento de um ponteiro.
typedef struct {
	assinador_ck_version version;
	void *antes_do_login[18];
	assinador_fn_login C_Login;
} assinador_lista_ate_o_login;

enum { ASSINADOR_OK = 0, ASSINADOR_SEM_MODULO = 1, ASSINADOR_SEM_LISTA = 2, ASSINADOR_SEM_MEMORIA = 3 };

// A lista de funções do módulo JÁ CARREGADO por quem abriu a sessão (o miekg/pkcs11). Com
// RTLD_NOLOAD o dlopen nunca carrega uma segunda cópia: ou devolve a mesma, ou falha.
static void *assinador_abrir(const char *modulo, assinador_lista_ate_o_login **lista) {
	void *h = dlopen(modulo, RTLD_LAZY | RTLD_NOLOAD);
	if (h == NULL) return NULL;
	assinador_fn_obter_lista obter = (assinador_fn_obter_lista) dlsym(h, "C_GetFunctionList");
	void *l = NULL;
	if (obter == NULL || obter(&l) != 0 || l == NULL) {
		dlclose(h);
		return NULL;
	}
	*lista = (assinador_lista_ate_o_login *) l;
	return h;
}

// C_Login com uma cópia do PIN em memória do C, zerada com escrita volátil ANTES de liberar. PIN
// vazio passa NULL com tamanho zero, que é o caminho protegido (o leitor ou o middleware pedem).
// `zerado` confirma, relendo, que a cópia foi apagada.
static int assinador_login(const char *modulo, assinador_ck_ulong sessao, assinador_ck_ulong usuario,
                           const assinador_ck_byte *pin, assinador_ck_ulong tamanho,
                           assinador_ck_ulong *rv, int *zerado) {
	assinador_lista_ate_o_login *lista = NULL;
	*zerado = 0;
	void *h = assinador_abrir(modulo, &lista);
	if (h == NULL) return ASSINADOR_SEM_MODULO;
	if (lista->C_Login == NULL) {
		dlclose(h);
		return ASSINADOR_SEM_LISTA;
	}
	assinador_ck_byte *copia = NULL;
	if (tamanho > 0) {
		copia = malloc(tamanho);
		if (copia == NULL) {
			dlclose(h);
			return ASSINADOR_SEM_MEMORIA;
		}
		memcpy(copia, pin, tamanho);
	}
	*rv = lista->C_Login(sessao, usuario, copia, tamanho);
	if (copia != NULL) {
		volatile assinador_ck_byte *v = copia;
		assinador_ck_ulong i;
		int limpo = 1;
		for (i = 0; i < tamanho; i++) v[i] = 0;
		for (i = 0; i < tamanho; i++) if (v[i] != 0) limpo = 0;
		*zerado = limpo;
		free(copia);
	} else {
		*zerado = 1;
	}
	dlclose(h);
	return ASSINADOR_OK;
}

// Para o teste: a lista do módulo tem C_GetFunctionList na posição do padrão? É a prova de que a
// estrutura acima tem o desenho certo nesta plataforma.
static int assinador_lista_coerente(const char *modulo) {
	assinador_lista_ate_o_login *lista = NULL;
	void *h = assinador_abrir(modulo, &lista);
	if (h == NULL) return 0;
	int ok = lista->antes_do_login[3] == dlsym(h, "C_GetFunctionList");
	dlclose(h);
	return ok;
}
*/
import "C"

import (
	"errors"
	"unsafe"

	p11 "github.com/miekg/pkcs11"
)

// Tipos de usuário do C_Login (PKCS#11: CKU_USER e CKU_CONTEXT_SPECIFIC).
const (
	usuarioComum      = p11.CKU_USER
	usuarioDaOperacao = p11.CKU_CONTEXT_SPECIFIC
)

// entrarNoToken chama `C_Login` do módulo `caminho` na sessão `sessao`, SEM o `Login` do
// miekg/pkcs11: aquele recebe `string`, copia para o C com `C.CString` e libera com `C.free` sem
// zerar, e uma cópia do PIN sobraria na memória do processo. Aqui a cópia é de `C.malloc`, zerada
// com escrita volátil antes do `free`. O `[]byte` de quem chama NÃO é tocado: quem chama o zera
// quando não precisar mais (pode haver um segundo login, `CKU_CONTEXT_SPECIFIC`).
//
// O módulo tem de estar carregado pelo `p11.Ctx` que abriu a sessão, pelo MESMO caminho.
// Devolve se a cópia foi confirmada zerada, e o erro do C_Login como `p11.Error`.
func entrarNoToken(caminho string, sessao p11.SessionHandle, usuario uint, pin []byte) (bool, error) {
	cCaminho := C.CString(caminho)
	defer C.free(unsafe.Pointer(cCaminho))
	var rv C.assinador_ck_ulong
	var zerado C.int
	var p *C.assinador_ck_byte
	if len(pin) > 0 {
		p = (*C.assinador_ck_byte)(unsafe.Pointer(&pin[0]))
	}
	switch C.assinador_login(cCaminho, C.assinador_ck_ulong(sessao), C.assinador_ck_ulong(usuario), p, C.assinador_ck_ulong(len(pin)), &rv, &zerado) {
	case C.ASSINADOR_OK:
	case C.ASSINADOR_SEM_MODULO:
		return true, errors.New("o módulo não está carregado neste processo")
	case C.ASSINADOR_SEM_LISTA:
		return true, errors.New("o módulo não tem C_Login")
	default:
		return true, errors.New("sem memória para o PIN")
	}
	if rv != 0 {
		return zerado == 1, p11.Error(rv)
	}
	return zerado == 1, nil
}

// listaCoerente é para o teste: confere o desenho da lista de funções nesta plataforma.
func listaCoerente(caminho string) bool {
	cCaminho := C.CString(caminho)
	defer C.free(unsafe.Pointer(cCaminho))
	return C.assinador_lista_coerente(cCaminho) == 1
}
