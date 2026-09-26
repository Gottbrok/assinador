// Package windows é o provedor de chaves do Windows (F6a, §3.5 do plano): lista o repositório
// pessoal do usuário (`CurrentUser\My`) e assina pela API do próprio sistema, CNG (KSP) ou CryptoAPI
// (CSP legado), sem PKCS#11 e sem cgo. O PIN é pedido pelo PROVEDOR do fabricante, num diálogo do
// Windows: a lista marca `exigePin: false`, e a janela da extensão não mostra campo de PIN.
//
// Este arquivo é a parte pura, que roda e se testa em qualquer sistema: o mapa dos códigos do
// Windows para o protocolo, a inversão de bytes do CSP e o rótulo do provedor. O que chama a API
// fica nos arquivos `_windows.go`.
package windows

import (
	"fmt"

	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// Códigos que o Windows devolve e que o protocolo distingue (winerror.h e scarderr.h). O teste de
// cabeçalho (`cabecalho_windows.go`, tag `windows_cabecalho`) os confere contra o SDK.
const (
	scardCancelado            = 0x80100002 // SCARD_E_CANCELLED
	scardSemCartao            = 0x8010000C // SCARD_E_NO_SMARTCARD
	scardCartaoRemovido       = 0x80100069 // SCARD_W_REMOVED_CARD
	scardPinErrado            = 0x8010006B // SCARD_W_WRONG_CHV
	scardPinBloqueado         = 0x8010006C // SCARD_W_CHV_BLOCKED
	scardCanceladoPeloUsuario = 0x8010006E // SCARD_W_CANCELLED_BY_USER
	nteAlgoritmo              = 0x80090008 // NTE_BAD_ALGID
	nteSemChave               = 0x8009000D // NTE_NO_KEY
	ntePermissao              = 0x80090010 // NTE_PERM
	nteConjuntoRuim           = 0x80090016 // NTE_BAD_KEYSET
	nteConjuntoNaoDefinido    = 0x80090019 // NTE_KEYSET_NOT_DEF
	nteNaoSuportado           = 0x80090029 // NTE_NOT_SUPPORTED
	nteCanceladoPeloUsuario   = 0x80090036 // NTE_USER_CANCELLED
	cryptSemChave             = 0x8009200B // CRYPT_E_NO_KEY_PROPERTY
	erroAcessoNegado          = 0x80070005 // HRESULT_FROM_WIN32(ERROR_ACCESS_DENIED)
	erroCancelado             = 0x800704C7 // HRESULT_FROM_WIN32(ERROR_CANCELLED)
)

// hresult põe o código no espaço do HRESULT: o `GetLastError` das funções da CryptoAPI devolve ora o
// NTE_* (já HRESULT), ora um erro do Win32 cru (ERROR_CANCELLED é 1223), e o mapa compara um só.
func hresult(codigo uint32) uint32 {
	if codigo != 0 && codigo <= 0xFFFF {
		return 0x80070000 | codigo
	}
	return codigo
}

// erroDoWindows traduz a recusa do Windows numa etapa do ato (`abrir a chave`, `assinar`) para o
// protocolo. O detalhe leva só a etapa e o código, para o suporte: nada do certificado nem da pessoa.
func erroDoWindows(etapa string, codigo uint32) *protocolo.Erro {
	h := hresult(codigo)
	detalhe := fmt.Sprintf("%s: 0x%08X", etapa, h)
	switch h {
	case scardCanceladoPeloUsuario, scardCancelado, nteCanceladoPeloUsuario, erroCancelado:
		return protocolo.Novo(protocolo.Cancelado, detalhe)
	case scardPinErrado:
		// O Windows não diz quantas tentativas restam: o `pin-incorreto` vai sem elas.
		return protocolo.PinErrado("")
	case scardPinBloqueado:
		return protocolo.Novo(protocolo.TokenBloqueado, detalhe)
	case nteAlgoritmo, nteNaoSuportado:
		return protocolo.Novo(protocolo.AlgoritmoNaoSuportado, detalhe+" (o provedor não assina SHA-256)")
	case scardSemCartao, scardCartaoRemovido, nteSemChave, nteConjuntoRuim, nteConjuntoNaoDefinido, cryptSemChave:
		return protocolo.Novo(protocolo.ChaveAusente, detalhe)
	case ntePermissao, erroAcessoNegado:
		return protocolo.Novo(protocolo.PermissaoNegada, detalhe)
	}
	return protocolo.Novo(protocolo.ModuloFalhou, detalhe)
}

// inverter troca a ordem dos bytes: o `CryptSignHash` do CSP legado devolve a assinatura em
// little-endian, e o RSA do protocolo (e de quem confere) é big-endian.
func inverter(b []byte) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}

// Rótulos que a lista mostra.
const (
	// RotuloInstalado é o certificado com a chave guardada no próprio Windows (o A1 importado, ou a
	// chave no TPM): ele também aparece e assina (§3.5 do plano).
	RotuloInstalado = "Certificado instalado no Windows"
	// RotuloSemNome é o provedor que não disse o nome.
	RotuloSemNome = "Cartão ou token"
)

// provedoresDoWindows são os provedores da Microsoft que guardam a chave no próprio computador, e não
// num cartão ou token. São nomes FIXOS da API (`MS_KEY_STORAGE_PROVIDER`, `MS_ENHANCED_PROV`,
// `MS_PLATFORM_CRYPTO_PROVIDER`...), e não medição: o do cartão (o "Microsoft Smart Card Key Storage
// Provider" e os dos fabricantes) aparece pelo próprio nome.
var provedoresDoWindows = map[string]bool{
	"Microsoft Software Key Storage Provider":               true,
	"Microsoft Platform Crypto Provider":                    true,
	"Microsoft Enhanced RSA and AES Cryptographic Provider": true,
	"Microsoft Enhanced Cryptographic Provider v1.0":        true,
	"Microsoft Strong Cryptographic Provider":               true,
	"Microsoft Base Cryptographic Provider v1.0":            true,
	"Microsoft RSA SChannel Cryptographic Provider":         true,
}

// rotuloDoProvedor é o que a lista mostra ao lado do certificado: "instalado no Windows" para a
// chave que mora no computador, e o nome do provedor para o cartão ou token.
func rotuloDoProvedor(nome string) string {
	switch {
	case provedoresDoWindows[nome]:
		return RotuloInstalado
	case nome == "":
		return RotuloSemNome
	}
	return nome
}

// Caminhos do provedor: `dwProvType` zero é chave CNG (KSP); qualquer outro é CSP legado.
const (
	CaminhoCNG = "cng"
	CaminhoCSP = "csp"
)

func caminhoDoTipo(tipoDoProvedor uint32) string {
	if tipoDoProvedor == 0 {
		return CaminhoCNG
	}
	return CaminhoCSP
}
