package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"math/big"
	"os"
	"strings"
	"testing"
)

// O DigestInfo montado à mão tem de ser o mesmo que o Go usa para PKCS#1 v1.5 com SHA-256: assinar
// o bloco "cru" (como o cartão faz com CKM_RSA_PKCS) e conferir com VerifyPKCS1v15 prova isso.
func TestDigestInfoConfereComPKCS1v15(t *testing.T) {
	chave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	resumo := sha256.Sum256([]byte("prova do Assinador"))
	bloco, err := DigestInfo(resumo[:])
	if err != nil {
		t.Fatal(err)
	}
	// Assinatura RSA "crua" sobre o bloco já montado, que é o que o CKM_RSA_PKCS do cartão faz.
	assinatura, err := rsa.SignPKCS1v15(nil, chave, crypto.Hash(0), bloco)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsa.VerifyPKCS1v15(&chave.PublicKey, crypto.SHA256, resumo[:], assinatura); err != nil {
		t.Fatalf("o DigestInfo não é o do PKCS#1 v1.5 com SHA-256: %v", err)
	}
}

func TestDigestInfoRecusaResumoDeOutroTamanho(t *testing.T) {
	if _, err := DigestInfo(make([]byte, 20)); err == nil {
		t.Fatal("aceitou resumo de 20 bytes")
	}
}

// O prefixo é o DER de DigestInfo{ AlgorithmIdentifier{ sha256, NULL }, OCTET STRING(32) }.
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
	derDoAsn1 := hex.EncodeToString(der[:len(prefixoDigestInfoSHA256)])
	prefixo := hex.EncodeToString(prefixoDigestInfoSHA256)
	if prefixo != derDoAsn1 {
		t.Fatalf("prefixo %s, o DER do asn1 começa com %s", prefixo, derDoAsn1)
	}
}

func TestMascararTiraOsDigitos(t *testing.T) {
	if got := Mascarar("MARIA DA SILVA:12345678901"); got != "MARIA DA SILVA:***********" {
		t.Fatalf("mascarado: %q", got)
	}
}

// lerPin fora do terminal lê a primeira linha da entrada padrão, sem guardar cópia em buffer.
func TestLerPinDaEntradaPadrao(t *testing.T) {
	casos := []struct {
		entrada string
		quer    string
		erro    bool
	}{
		{"1234\n", "1234", false},
		{"1234\r\nresto", "1234", false},
		{"98765", "98765", false},
		{"", "", true},
		{strings.Repeat("9", tetoDoPin+1) + "\n", "", true},
	}
	original := os.Stdin
	defer func() { os.Stdin = original }()
	for _, c := range casos {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.WriteString(c.entrada)
		_ = w.Close()
		os.Stdin = r
		pin, err := lerPin()
		_ = r.Close()
		if c.erro {
			if err == nil {
				t.Fatalf("%q: esperava erro", c.entrada)
			}
			continue
		}
		if err != nil || string(pin) != c.quer {
			t.Fatalf("%q: pin %q, erro %v", c.entrada, pin, err)
		}
	}
}

func TestRefEhSha256DoDerEmHexMinusculo(t *testing.T) {
	der := big.NewInt(42).Bytes()
	h := sha256.Sum256(der)
	if Ref(der) != hex.EncodeToString(h[:]) {
		t.Fatal("ref diferente do SHA-256 do DER")
	}
}
