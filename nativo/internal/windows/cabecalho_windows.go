//go:build windows && windows_cabecalho

package windows

/*
#include <stddef.h>
#include <wchar.h>
#include <windows.h>
#include <wincrypt.h>
#include <winscard.h>
#include <bcrypt.h>
#include <ncrypt.h>

static unsigned long codigo(HRESULT h) { return (unsigned long)(unsigned int)h; }

static unsigned long real_scard_cancelado(void) { return codigo(SCARD_E_CANCELLED); }
static unsigned long real_scard_sem_cartao(void) { return codigo(SCARD_E_NO_SMARTCARD); }
static unsigned long real_scard_removido(void) { return codigo(SCARD_W_REMOVED_CARD); }
static unsigned long real_scard_pin_errado(void) { return codigo(SCARD_W_WRONG_CHV); }
static unsigned long real_scard_pin_bloqueado(void) { return codigo(SCARD_W_CHV_BLOCKED); }
static unsigned long real_scard_cancelado_pelo_usuario(void) { return codigo(SCARD_W_CANCELLED_BY_USER); }
static unsigned long real_nte_algoritmo(void) { return codigo(NTE_BAD_ALGID); }
static unsigned long real_nte_sem_chave(void) { return codigo(NTE_NO_KEY); }
static unsigned long real_nte_permissao(void) { return codigo(NTE_PERM); }
static unsigned long real_nte_conjunto(void) { return codigo(NTE_BAD_KEYSET); }
static unsigned long real_nte_conjunto_nao_definido(void) { return codigo(NTE_KEYSET_NOT_DEF); }
static unsigned long real_nte_nao_suportado(void) { return codigo(NTE_NOT_SUPPORTED); }
static unsigned long real_nte_cancelado(void) { return codigo(NTE_USER_CANCELLED); }
static unsigned long real_crypt_sem_chave(void) { return codigo(CRYPT_E_NO_KEY_PROPERTY); }
static unsigned long real_acesso_negado(void) { return codigo(HRESULT_FROM_WIN32(ERROR_ACCESS_DENIED)); }
static unsigned long real_erro_cancelado(void) { return codigo(HRESULT_FROM_WIN32(ERROR_CANCELLED)); }

static unsigned long real_prop_info(void) { return CERT_KEY_PROV_INFO_PROP_ID; }
static unsigned long real_pp_hwnd(void) { return PP_CLIENT_HWND; }
static unsigned long real_calg_sha256(void) { return CALG_SHA_256; }
static unsigned long real_hp_hashval(void) { return HP_HASHVAL; }
static unsigned long real_pad_pkcs1(void) { return BCRYPT_PAD_PKCS1; }
static int real_propriedade_da_janela(void) { return wcscmp(NCRYPT_WINDOW_HANDLE_PROPERTY, L"HWND Handle") == 0; }
static int real_sha256(void) { return wcscmp(BCRYPT_SHA256_ALGORITHM, L"SHA256") == 0; }

static size_t real_tam_info(void) { return sizeof(CRYPT_KEY_PROV_INFO); }
static size_t real_des_container(void) { return offsetof(CRYPT_KEY_PROV_INFO, pwszContainerName); }
static size_t real_des_provedor(void) { return offsetof(CRYPT_KEY_PROV_INFO, pwszProvName); }
static size_t real_des_tipo(void) { return offsetof(CRYPT_KEY_PROV_INFO, dwProvType); }
static size_t real_des_keyspec(void) { return offsetof(CRYPT_KEY_PROV_INFO, dwKeySpec); }
static size_t real_tam_padding(void) { return sizeof(BCRYPT_PKCS1_PADDING_INFO); }
*/
import "C"

import "unsafe"

// comparacaoComOCabecalho devolve, para cada medida, o valor do SDK e o nosso.
func comparacaoComOCabecalho() map[string][2]uint64 {
	var info infoDoProvedorDaChave
	var padding bcryptPKCS1PaddingInfo
	// Os dois nomes em UTF-16 são conferidos no C (`wcscmp`), que devolve 1 quando são iguais.
	const igual = 1
	return map[string][2]uint64{
		"SCARD_E_CANCELLED":                           {uint64(C.real_scard_cancelado()), scardCancelado},
		"SCARD_E_NO_SMARTCARD":                        {uint64(C.real_scard_sem_cartao()), scardSemCartao},
		"SCARD_W_REMOVED_CARD":                        {uint64(C.real_scard_removido()), scardCartaoRemovido},
		"SCARD_W_WRONG_CHV":                           {uint64(C.real_scard_pin_errado()), scardPinErrado},
		"SCARD_W_CHV_BLOCKED":                         {uint64(C.real_scard_pin_bloqueado()), scardPinBloqueado},
		"SCARD_W_CANCELLED_BY_USER":                   {uint64(C.real_scard_cancelado_pelo_usuario()), scardCanceladoPeloUsuario},
		"NTE_BAD_ALGID":                               {uint64(C.real_nte_algoritmo()), nteAlgoritmo},
		"NTE_NO_KEY":                                  {uint64(C.real_nte_sem_chave()), nteSemChave},
		"NTE_PERM":                                    {uint64(C.real_nte_permissao()), ntePermissao},
		"NTE_BAD_KEYSET":                              {uint64(C.real_nte_conjunto()), nteConjuntoRuim},
		"NTE_KEYSET_NOT_DEF":                          {uint64(C.real_nte_conjunto_nao_definido()), nteConjuntoNaoDefinido},
		"NTE_NOT_SUPPORTED":                           {uint64(C.real_nte_nao_suportado()), nteNaoSuportado},
		"NTE_USER_CANCELLED":                          {uint64(C.real_nte_cancelado()), nteCanceladoPeloUsuario},
		"CRYPT_E_NO_KEY_PROPERTY":                     {uint64(C.real_crypt_sem_chave()), cryptSemChave},
		"HRESULT_FROM_WIN32(ERROR_ACCESS_DENIED)":     {uint64(C.real_acesso_negado()), erroAcessoNegado},
		"HRESULT_FROM_WIN32(ERROR_CANCELLED)":         {uint64(C.real_erro_cancelado()), erroCancelado},
		"CERT_KEY_PROV_INFO_PROP_ID":                  {uint64(C.real_prop_info()), certKeyProvInfoPropID},
		"PP_CLIENT_HWND":                              {uint64(C.real_pp_hwnd()), ppClientHwnd},
		"CALG_SHA_256":                                {uint64(C.real_calg_sha256()), calgSHA256},
		"HP_HASHVAL":                                  {uint64(C.real_hp_hashval()), hpHashVal},
		"BCRYPT_PAD_PKCS1":                            {uint64(C.real_pad_pkcs1()), bcryptPadPKCS1},
		"NCRYPT_WINDOW_HANDLE_PROPERTY é HWND Handle": {uint64(C.real_propriedade_da_janela()), igual},
		"BCRYPT_SHA256_ALGORITHM é SHA256":            {uint64(C.real_sha256()), igual},
		"sizeof(CRYPT_KEY_PROV_INFO)":                 {uint64(C.real_tam_info()), uint64(unsafe.Sizeof(info))},
		"offsetof(pwszContainerName)":                 {uint64(C.real_des_container()), uint64(unsafe.Offsetof(info.container))},
		"offsetof(pwszProvName)":                      {uint64(C.real_des_provedor()), uint64(unsafe.Offsetof(info.provedor))},
		"offsetof(dwProvType)":                        {uint64(C.real_des_tipo()), uint64(unsafe.Offsetof(info.tipo))},
		"offsetof(dwKeySpec)":                         {uint64(C.real_des_keyspec()), uint64(unsafe.Offsetof(info.keySpec))},
		"sizeof(BCRYPT_PKCS1_PADDING_INFO)":           {uint64(C.real_tam_padding()), uint64(unsafe.Sizeof(padding))},
	}
}
