//go:build windows

package windows

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"

	win "golang.org/x/sys/windows"
)

// As funções que o `golang.org/x/sys/windows` não embrulha. `NewLazySystemDLL` só carrega a DLL da
// pasta do sistema (System32): uma DLL de mesmo nome na pasta do programa ou no PATH não entra.
var (
	crypt32  = win.NewLazySystemDLL("crypt32.dll")
	advapi32 = win.NewLazySystemDLL("advapi32.dll")
	ncrypt   = win.NewLazySystemDLL("ncrypt.dll")

	procCertGetCertificateContextProperty = crypt32.NewProc("CertGetCertificateContextProperty")
	procCryptSetProvParam                 = advapi32.NewProc("CryptSetProvParam")
	procCryptCreateHash                   = advapi32.NewProc("CryptCreateHash")
	procCryptSetHashParam                 = advapi32.NewProc("CryptSetHashParam")
	procCryptSignHashW                    = advapi32.NewProc("CryptSignHashW")
	procCryptDestroyHash                  = advapi32.NewProc("CryptDestroyHash")
	procNCryptSetProperty                 = ncrypt.NewProc("NCryptSetProperty")
	procNCryptSignHash                    = ncrypt.NewProc("NCryptSignHash")
	procNCryptFreeObject                  = ncrypt.NewProc("NCryptFreeObject")
)

// Constantes da API (wincrypt.h, ncrypt.h, bcrypt.h).
const (
	certKeyProvInfoPropID = 2      // CERT_KEY_PROV_INFO_PROP_ID
	ppClientHwnd          = 1      // PP_CLIENT_HWND
	calgSHA256            = 0x800c // CALG_SHA_256
	hpHashVal             = 2      // HP_HASHVAL
	bcryptPadPKCS1        = 2      // BCRYPT_PAD_PKCS1
	// tamanhoMaximoDaAssinatura cabe o RSA de 8192 bits: a ICP-Brasil emite 2048 e 4096. Uma chamada
	// só, com o buffer já do tamanho, porque há provedor que pede o PIN também na consulta de tamanho.
	tamanhoMaximoDaAssinatura = 1024
)

// infoDoProvedorDaChave é o CRYPT_KEY_PROV_INFO: onde mora a chave do certificado. Os ponteiros
// apontam para dentro do mesmo bloco que o Windows preencheu.
type infoDoProvedorDaChave struct {
	container *uint16
	provedor  *uint16
	tipo      uint32
	flags     uint32
	nParams   uint32
	params    uintptr
	keySpec   uint32
}

// bcryptPKCS1PaddingInfo é o BCRYPT_PKCS1_PADDING_INFO: o algoritmo do resumo, para o CNG montar o
// DigestInfo.
type bcryptPKCS1PaddingInfo struct {
	algoritmo *uint16
}

// codigoDoErro tira o código numérico de um erro do syscall (`GetLastError`).
func codigoDoErro(err error) uint32 {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return uint32(errno)
	}
	return 0
}

// provedorDaChave lê o CERT_KEY_PROV_INFO do certificado, SEM abrir a chave (listar nunca pede PIN):
// o nome do provedor e o tipo dele. Certificado sem a propriedade não tem chave privada associada.
func provedorDaChave(ctx *win.CertContext) (nome string, tipo uint32, ok bool) {
	var tamanho uint32
	r, _, _ := procCertGetCertificateContextProperty.Call(uintptr(unsafe.Pointer(ctx)), certKeyProvInfoPropID, 0, uintptr(unsafe.Pointer(&tamanho)))
	if r == 0 || tamanho < uint32(unsafe.Sizeof(infoDoProvedorDaChave{})) {
		return "", 0, false
	}
	// Em palavras de 8 bytes, para o bloco vir alinhado como a estrutura.
	bloco := make([]uint64, (tamanho+7)/8)
	r, _, _ = procCertGetCertificateContextProperty.Call(uintptr(unsafe.Pointer(ctx)), certKeyProvInfoPropID, uintptr(unsafe.Pointer(&bloco[0])), uintptr(unsafe.Pointer(&tamanho)))
	if r == 0 {
		return "", 0, false
	}
	info := (*infoDoProvedorDaChave)(unsafe.Pointer(&bloco[0]))
	nome = win.UTF16PtrToString(info.provedor)
	tipo = info.tipo
	runtime.KeepAlive(bloco)
	return nome, tipo, true
}

// definirJanelaDoCSP diz aos provedores CSP deste processo qual janela é a mãe dos diálogos deles. É
// global (hProv nulo) e vem ANTES de adquirir a chave, porque há CSP que abre diálogo já ali.
func definirJanelaDoCSP(janela uintptr) error {
	r, _, err := procCryptSetProvParam.Call(0, ppClientHwnd, uintptr(unsafe.Pointer(&janela)), 0)
	if r == 0 {
		return err
	}
	return nil
}

// definirJanelaDaChaveCNG diz ao provedor CNG qual janela é a mãe do diálogo de PIN.
func definirJanelaDaChaveCNG(chave win.Handle, janela uintptr) error {
	nome, _ := win.UTF16PtrFromString("HWND Handle") // NCRYPT_WINDOW_HANDLE_PROPERTY
	r, _, _ := procNCryptSetProperty.Call(uintptr(chave), uintptr(unsafe.Pointer(nome)), uintptr(unsafe.Pointer(&janela)), unsafe.Sizeof(janela), 0)
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

// assinarCNG assina o resumo SHA-256 com a chave CNG: o próprio CNG monta o DigestInfo (PKCS#1 v1.5
// com o algoritmo do padding), e o diálogo de PIN, se houver, é do provedor.
func assinarCNG(chave win.Handle, digest [32]byte) ([]byte, uint32) {
	algoritmo, _ := win.UTF16PtrFromString("SHA256") // BCRYPT_SHA256_ALGORITHM
	padding := bcryptPKCS1PaddingInfo{algoritmo: algoritmo}
	assinatura := make([]byte, tamanhoMaximoDaAssinatura)
	var tamanho uint32
	r, _, _ := procNCryptSignHash.Call(
		uintptr(chave),
		uintptr(unsafe.Pointer(&padding)),
		uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)),
		uintptr(unsafe.Pointer(&assinatura[0])), uintptr(len(assinatura)),
		uintptr(unsafe.Pointer(&tamanho)),
		bcryptPadPKCS1,
	)
	runtime.KeepAlive(algoritmo)
	if r != 0 {
		return nil, uint32(r)
	}
	return assinatura[:tamanho], 0
}

// liberarChaveCNG fecha a chave CNG que o `CryptAcquireCertificatePrivateKey` entregou.
func liberarChaveCNG(chave win.Handle) {
	_, _, _ = procNCryptFreeObject.Call(uintptr(chave))
}

// assinarCSP assina o resumo SHA-256 com a chave do CSP legado: o hash é criado com o valor pronto
// (HP_HASHVAL), e o CSP monta o DigestInfo pelo algoritmo do hash. O CSP devolve a assinatura em
// little-endian, e ela sai invertida. CSP sem SHA-256 recusa o `CryptCreateHash` com NTE_BAD_ALGID.
func assinarCSP(provedor win.Handle, keySpec uint32, digest [32]byte) ([]byte, string, uint32) {
	var hash uintptr
	r, _, err := procCryptCreateHash.Call(uintptr(provedor), calgSHA256, 0, 0, uintptr(unsafe.Pointer(&hash)))
	if r == 0 {
		return nil, "criar o resumo", codigoDoErro(err)
	}
	defer procCryptDestroyHash.Call(hash)
	r, _, err = procCryptSetHashParam.Call(hash, hpHashVal, uintptr(unsafe.Pointer(&digest[0])), 0)
	if r == 0 {
		return nil, "dar o resumo", codigoDoErro(err)
	}
	assinatura := make([]byte, tamanhoMaximoDaAssinatura)
	tamanho := uint32(len(assinatura))
	r, _, err = procCryptSignHashW.Call(hash, uintptr(keySpec), 0, 0, uintptr(unsafe.Pointer(&assinatura[0])), uintptr(unsafe.Pointer(&tamanho)))
	if r == 0 {
		return nil, "assinar", codigoDoErro(err)
	}
	assinatura = assinatura[:tamanho]
	inverter(assinatura)
	return assinatura, "", 0
}
