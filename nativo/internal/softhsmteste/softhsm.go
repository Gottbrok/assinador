//go:build !windows

// Package softhsmteste monta, para os TESTES, um token SoftHSM2 num diretório temporário, com
// chaves e certificados de teste que fazem o papel de um cartão A3. Não entra no programa: só
// arquivos `_test.go` o importam (a catraca de `cmd/assinador/release_test.go` confere o binário).
//
// O módulo é procurado em `ASSINADOR_SOFTHSM` e nos caminhos comuns. Sem ele, o teste é PULADO,
// a não ser que `ASSINADOR_EXIGE_SOFTHSM=1` (o CI liga), e aí ele reprova.
//
// Fora do Windows: o `miekg/pkcs11` não compila lá, e o provedor do Windows não é PKCS#11.
package softhsmteste

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	p11 "github.com/miekg/pkcs11"
)

// PIN e SOPIN do token de teste. São de TESTE: o token vive num diretório temporário.
const (
	PIN   = "123456"
	SOPIN = "87654321"
)

// Nomes dos certificados que o token tem.
const (
	Titular         = "titular"          // RSA, uso de assinatura, válido: o que assina
	Vencido         = "vencido"          // RSA com chave, vencido: listado, nunca assinado
	Eliptica        = "eliptica"         // chave EC: fora da lista (só RSA na v1)
	AC              = "ac"               // de AC, sem chave: fora da lista
	SempreAutentica = "sempre-autentica" // RSA com CKA_ALWAYS_AUTHENTICATE: pede o PIN de novo na operação
)

// CN do titular de teste, no formato ICP-Brasil `NOME:CPF`.
const CNDoTitular = "TITULAR DE TESTE:12345678901"

// Opcoes do token.
type Opcoes struct {
	// ChavesVisiveis cria as chaves privadas com CKA_PRIVATE falso: o token as mostra antes do login.
	ChavesVisiveis bool
}

// Token é um token SoftHSM2 pronto.
type Token struct {
	Modulo       string
	Conf         string
	Certificados map[string]*x509.Certificate
}

// Modulo acha a biblioteca do SoftHSM2, ou pula (ou reprova, no CI) o teste.
func Modulo(t testing.TB) string {
	t.Helper()
	candidatos := []string{
		os.Getenv("ASSINADOR_SOFTHSM"),
		"/usr/lib/softhsm/libsofthsm2.so",
		"/usr/lib/x86_64-linux-gnu/softhsm/libsofthsm2.so",
		"/usr/lib/aarch64-linux-gnu/softhsm/libsofthsm2.so",
		"/usr/lib64/pkcs11/libsofthsm2.so",
		"/usr/local/lib/softhsm/libsofthsm2.so",
	}
	for _, c := range candidatos {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if os.Getenv("ASSINADOR_EXIGE_SOFTHSM") == "1" {
		t.Fatal("SoftHSM2 não encontrado e ASSINADOR_EXIGE_SOFTHSM=1")
	}
	t.Skip("SoftHSM2 não encontrado (defina ASSINADOR_SOFTHSM)")
	return ""
}

// Novo cria o token num diretório temporário e aponta SOFTHSM2_CONF para ele (os processos filhos
// herdam). Usa t.Setenv: o teste que o chama não pode ser paralelo.
func Novo(t testing.TB, o Opcoes) *Token {
	t.Helper()
	modulo := Modulo(t)
	dir := t.TempDir()
	tokens := filepath.Join(dir, "tokens")
	if err := os.Mkdir(tokens, 0o700); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(dir, "softhsm2.conf")
	conteudo := "directories.tokendir = " + tokens + "\nobjectstore.backend = file\nlog.level = ERROR\nslots.removable = false\n"
	if err := os.WriteFile(conf, []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOFTHSM2_CONF", conf)

	ctx := p11.New(modulo)
	if ctx == nil {
		t.Fatalf("SoftHSM2 não carregou: %s", modulo)
	}
	if err := ctx.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = ctx.Finalize()
		ctx.Destroy()
	}()
	slots, err := ctx.GetSlotList(false)
	if err != nil || len(slots) == 0 {
		t.Fatalf("sem slot: %v", err)
	}
	const rotulo = "cartao de teste"
	if err := ctx.InitToken(slots[0], SOPIN, rotulo); err != nil {
		t.Fatal(err)
	}
	// O SoftHSM2 renumera o slot depois de inicializar o token.
	slot := slotDoRotulo(t, ctx, rotulo)
	s, err := ctx.OpenSession(slot, p11.CKF_SERIAL_SESSION|p11.CKF_RW_SESSION)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ctx.CloseSession(s) }()
	if err := ctx.Login(s, p11.CKU_SO, SOPIN); err != nil {
		t.Fatal(err)
	}
	if err := ctx.InitPIN(s, PIN); err != nil {
		t.Fatal(err)
	}
	_ = ctx.Logout(s)
	if err := ctx.Login(s, p11.CKU_USER, PIN); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ctx.Logout(s) }()

	acKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	agora := time.Now()
	acModelo := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "AC DE TESTE DO ASSINADOR"},
		NotBefore: agora.Add(-time.Hour), NotAfter: agora.Add(365 * 24 * time.Hour),
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, BasicConstraintsValid: true, IsCA: true,
	}
	acDer, err := x509.CreateCertificate(rand.Reader, acModelo, acModelo, &acKey.PublicKey, acKey)
	if err != nil {
		t.Fatal(err)
	}
	acCert, _ := x509.ParseCertificate(acDer)
	tk := &Token{Modulo: modulo, Conf: conf, Certificados: map[string]*x509.Certificate{}}

	emitir := func(nome string, id []byte, publica any, de, ate time.Time) {
		modelo := &x509.Certificate{
			SerialNumber: big.NewInt(int64(len(tk.Certificados) + 2)),
			Subject:      pkix.Name{CommonName: CNDoTitular},
			NotBefore:    de, NotAfter: ate,
			KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		}
		der, err := x509.CreateCertificate(rand.Reader, modelo, acCert, publica, acKey)
		if err != nil {
			t.Fatal(err)
		}
		c, _ := x509.ParseCertificate(der)
		guardarCertificado(t, ctx, s, id, nome, der, c.RawSubject)
		tk.Certificados[nome] = c
	}

	privada := !o.ChavesVisiveis
	emitir(Titular, []byte{0x01}, parRSA(t, ctx, s, []byte{0x01}, privada, false), agora.Add(-time.Hour), agora.Add(365*24*time.Hour))
	emitir(Vencido, []byte{0x02}, parRSA(t, ctx, s, []byte{0x02}, privada, false), agora.Add(-48*time.Hour), agora.Add(-24*time.Hour))
	emitir(Eliptica, []byte{0x03}, parEC(t, ctx, s, []byte{0x03}, privada), agora.Add(-time.Hour), agora.Add(365*24*time.Hour))
	// CKA_ALWAYS_AUTHENTICATE só existe em chave privada (o SoftHSM recusa o modelo sem ela): esta
	// fica escondida até o login mesmo no token de chaves visíveis, como acontece em cartão real.
	emitir(SempreAutentica, []byte{0x04}, parRSA(t, ctx, s, []byte{0x04}, true, true), agora.Add(-time.Hour), agora.Add(365*24*time.Hour))
	guardarCertificado(t, ctx, s, []byte{0xca}, AC, acDer, acCert.RawSubject)
	tk.Certificados[AC] = acCert
	return tk
}

func slotDoRotulo(t testing.TB, ctx *p11.Ctx, rotulo string) uint {
	t.Helper()
	slots, err := ctx.GetSlotList(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range slots {
		if ti, err := ctx.GetTokenInfo(s); err == nil && strings.TrimSpace(ti.Label) == rotulo {
			return s
		}
	}
	t.Fatal("token recém-criado não apareceu")
	return 0
}

func parRSA(t testing.TB, ctx *p11.Ctx, s p11.SessionHandle, id []byte, privada, sempreAutentica bool) *rsa.PublicKey {
	t.Helper()
	pub, _, err := ctx.GenerateKeyPair(s, []*p11.Mechanism{p11.NewMechanism(p11.CKM_RSA_PKCS_KEY_PAIR_GEN, nil)},
		[]*p11.Attribute{
			p11.NewAttribute(p11.CKA_TOKEN, true),
			p11.NewAttribute(p11.CKA_VERIFY, true),
			p11.NewAttribute(p11.CKA_MODULUS_BITS, 2048),
			p11.NewAttribute(p11.CKA_PUBLIC_EXPONENT, []byte{1, 0, 1}),
			p11.NewAttribute(p11.CKA_ID, id),
		},
		[]*p11.Attribute{
			p11.NewAttribute(p11.CKA_TOKEN, true),
			p11.NewAttribute(p11.CKA_PRIVATE, privada),
			p11.NewAttribute(p11.CKA_SENSITIVE, true),
			p11.NewAttribute(p11.CKA_SIGN, true),
			p11.NewAttribute(p11.CKA_ID, id),
			p11.NewAttribute(p11.CKA_ALWAYS_AUTHENTICATE, sempreAutentica),
		})
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := ctx.GetAttributeValue(s, pub, []*p11.Attribute{p11.NewAttribute(p11.CKA_MODULUS, nil), p11.NewAttribute(p11.CKA_PUBLIC_EXPONENT, nil)})
	if err != nil {
		t.Fatal(err)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(attrs[0].Value), E: int(new(big.Int).SetBytes(attrs[1].Value).Int64())}
}

func parEC(t testing.TB, ctx *p11.Ctx, s p11.SessionHandle, id []byte, privada bool) *ecdsa.PublicKey {
	t.Helper()
	p256, _ := asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 10045, 3, 1, 7})
	pub, _, err := ctx.GenerateKeyPair(s, []*p11.Mechanism{p11.NewMechanism(p11.CKM_EC_KEY_PAIR_GEN, nil)},
		[]*p11.Attribute{
			p11.NewAttribute(p11.CKA_TOKEN, true),
			p11.NewAttribute(p11.CKA_VERIFY, true),
			p11.NewAttribute(p11.CKA_EC_PARAMS, p256),
			p11.NewAttribute(p11.CKA_ID, id),
		},
		[]*p11.Attribute{
			p11.NewAttribute(p11.CKA_TOKEN, true),
			p11.NewAttribute(p11.CKA_PRIVATE, privada),
			p11.NewAttribute(p11.CKA_SENSITIVE, true),
			p11.NewAttribute(p11.CKA_SIGN, true),
			p11.NewAttribute(p11.CKA_ID, id),
		})
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := ctx.GetAttributeValue(s, pub, []*p11.Attribute{p11.NewAttribute(p11.CKA_EC_POINT, nil)})
	if err != nil {
		t.Fatal(err)
	}
	var ponto []byte
	if _, err := asn1.Unmarshal(attrs[0].Value, &ponto); err != nil {
		t.Fatal(err)
	}
	chave, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), ponto)
	if err != nil {
		t.Fatal(err)
	}
	return chave
}

func guardarCertificado(t testing.TB, ctx *p11.Ctx, s p11.SessionHandle, id []byte, rotulo string, der, sujeito []byte) {
	t.Helper()
	_, err := ctx.CreateObject(s, []*p11.Attribute{
		p11.NewAttribute(p11.CKA_CLASS, p11.CKO_CERTIFICATE),
		p11.NewAttribute(p11.CKA_CERTIFICATE_TYPE, p11.CKC_X_509),
		p11.NewAttribute(p11.CKA_TOKEN, true),
		p11.NewAttribute(p11.CKA_ID, id),
		p11.NewAttribute(p11.CKA_LABEL, rotulo),
		p11.NewAttribute(p11.CKA_SUBJECT, sujeito),
		p11.NewAttribute(p11.CKA_VALUE, der),
	})
	if err != nil {
		t.Fatal(err)
	}
}
