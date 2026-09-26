package assinatura

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// O DigestInfo montado à mão é o que o Go usa em PKCS#1 v1.5 com SHA-256: assinar o bloco "cru"
// (como o cartão faz com CKM_RSA_PKCS) e conferir com VerifyPKCS1v15 prova isso.
func TestDigestInfoConfereComPKCS1v15(t *testing.T) {
	chave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	resumo := sha256.Sum256([]byte("prova do Assinador"))
	assinatura, err := rsa.SignPKCS1v15(nil, chave, crypto.Hash(0), DigestInfo(resumo))
	if err != nil {
		t.Fatal(err)
	}
	if err := rsa.VerifyPKCS1v15(&chave.PublicKey, crypto.SHA256, resumo[:], assinatura); err != nil {
		t.Fatalf("o DigestInfo não é o do PKCS#1 v1.5 com SHA-256: %v", err)
	}
}

func TestPrefixoEhODerDoDigestInfo(t *testing.T) {
	type algoritmo struct {
		OID    asn1.ObjectIdentifier
		Params asn1.RawValue
	}
	type digestInfo struct {
		Algoritmo algoritmo
		Resumo    []byte
	}
	der, err := asn1.Marshal(digestInfo{
		Algoritmo: algoritmo{OID: asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}, Params: asn1.NullRawValue},
		Resumo:    make([]byte, 32),
	})
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(der) != hex.EncodeToString(DigestInfo([32]byte{})) {
		t.Fatalf("DigestInfo %x, o asn1 dá %x", DigestInfo([32]byte{}), der)
	}
}

func certificado(t *testing.T, publica any, uso x509.KeyUsage, ca bool, de, ate time.Time) *x509.Certificate {
	t.Helper()
	emissor, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	modelo := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "TITULAR DE TESTE:12345678901"},
		NotBefore:             de,
		NotAfter:              ate,
		KeyUsage:              uso,
		BasicConstraintsValid: true,
		IsCA:                  ca,
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, publica, emissor)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestListavelEAssinavel(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	agora := time.Now()
	ontem, amanha := agora.Add(-24*time.Hour), agora.Add(24*time.Hour)
	assinar := x509.KeyUsageDigitalSignature
	naoRepudio := x509.KeyUsageContentCommitment

	casos := []struct {
		nome      string
		c         *x509.Certificate
		listavel  bool
		assinavel protocolo.Codigo // vazio: pode assinar
	}{
		{"assinatura digital", certificado(t, &rsaKey.PublicKey, assinar, false, ontem, amanha), true, ""},
		{"não repúdio", certificado(t, &rsaKey.PublicKey, naoRepudio, false, ontem, amanha), true, ""},
		{"de AC", certificado(t, &rsaKey.PublicKey, x509.KeyUsageCertSign|assinar, true, ontem, amanha), false, protocolo.CertificadoNaoEncontrado},
		{"só cifra", certificado(t, &rsaKey.PublicKey, x509.KeyUsageKeyEncipherment, false, ontem, amanha), false, protocolo.CertificadoNaoEncontrado},
		{"curva elíptica", certificado(t, &ecKey.PublicKey, assinar, false, ontem, amanha), false, protocolo.AlgoritmoNaoSuportado},
		{"vencido", certificado(t, &rsaKey.PublicKey, assinar, false, agora.Add(-48*time.Hour), ontem), true, protocolo.CertificadoNaoEncontrado},
		{"futuro", certificado(t, &rsaKey.PublicKey, assinar, false, amanha, amanha.Add(time.Hour)), true, protocolo.CertificadoNaoEncontrado},
	}
	for _, c := range casos {
		if ok, _ := Listavel(c.c); ok != c.listavel {
			t.Errorf("%s: listável = %v", c.nome, ok)
		}
		e := Assinavel(c.c, agora)
		if (e == nil) != (c.assinavel == "") || (e != nil && e.Codigo != c.assinavel) {
			t.Errorf("%s: assinável = %v", c.nome, e)
		}
	}
}

func TestConferirAssinaturaPegaChaveTrocada(t *testing.T) {
	certKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	outraKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	c := certificado(t, &certKey.PublicKey, x509.KeyUsageDigitalSignature, false, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	resumo := sha256.Sum256([]byte("x"))
	boa, _ := rsa.SignPKCS1v15(nil, certKey, crypto.Hash(0), DigestInfo(resumo))
	if err := ConferirAssinatura(c, resumo, boa); err != nil {
		t.Fatalf("recusou a boa: %v", err)
	}
	trocada, _ := rsa.SignPKCS1v15(nil, outraKey, crypto.Hash(0), DigestInfo(resumo))
	if ConferirAssinatura(c, resumo, trocada) == nil {
		t.Fatal("aceitou assinatura de outra chave")
	}
}

func TestMascarar(t *testing.T) {
	if got := Mascarar("MARIA DA SILVA:12345678901"); got != "MARIA DA SILVA:***********" {
		t.Fatalf("%q", got)
	}
}
