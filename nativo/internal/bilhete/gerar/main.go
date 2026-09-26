// Gera `internal/bilhete/chaves.go` a partir de `protocolo/chaves-publicas.json`:
//
//	go generate ./internal/bilhete
//
// O JSON lista as chaves PÚBLICAS dos emissores de bilhete, na forma
// `{ kid, iss, ambiente, jwk: { kty, crv, x, y } }`. Só entra chave de ambiente `producao`: o
// gerador RECUSA qualquer outra, porque o que está em `chaves.go` vai para o build de release
// (regra 5 do CLAUDE.md). Chave de teste mora nas fixtures, só para os testes; chave de
// desenvolvimento mora no arquivo local, só para o build `dev`.
package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"regexp"

	"github.com/Gottbrok/assinador/nativo/internal/origem"
)

type entrada struct {
	Kid      string `json:"kid"`
	Iss      string `json:"iss"`
	Ambiente string `json:"ambiente"`
	Jwk      struct {
		Kty string `json:"kty"`
		Crv string `json:"crv"`
		X   string `json:"x"`
		Y   string `json:"y"`
	} `json:"jwk"`
}

var (
	formatoDoKid = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
	alfabeto     = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

func main() {
	caminhoDeEntrada := flag.String("entrada", "", "protocolo/chaves-publicas.json")
	caminhoDeSaida := flag.String("saida", "", "internal/bilhete/chaves.go")
	flag.Parse()
	bruto, err := os.ReadFile(*caminhoDeEntrada)
	if err != nil {
		falhar(err)
	}
	codigo, err := gerar(bruto)
	if err != nil {
		falhar(err)
	}
	if err := os.WriteFile(*caminhoDeSaida, codigo, 0o644); err != nil {
		falhar(err)
	}
}

func falhar(err error) {
	fmt.Fprintln(os.Stderr, "gerar chaves:", err)
	os.Exit(1)
}

func gerar(bruto []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(bruto))
	dec.DisallowUnknownFields()
	var entradas []entrada
	if err := dec.Decode(&entradas); err != nil {
		return nil, fmt.Errorf("o arquivo não é uma lista de chaves: %w", err)
	}
	if dec.More() {
		return nil, errors.New("conteúdo depois da lista")
	}
	vistos := map[string]bool{}
	var b bytes.Buffer
	b.WriteString("// Código GERADO por `go generate ./internal/bilhete` a partir de protocolo/chaves-publicas.json.\n")
	b.WriteString("// NÃO EDITE À MÃO: mude o JSON e gere de novo. O teste do gerador reprova arquivo desatualizado.\n\n")
	b.WriteString("package bilhete\n\n")
	b.WriteString("// chavesPinadas são as chaves de PRODUÇÃO em que o programa acredita. O gerador recusa entrada\n")
	b.WriteString("// de outro ambiente: o build de release não leva chave dev nem teste.\n")
	b.WriteString("var chavesPinadas = []jwkPinada{\n")
	for _, e := range entradas {
		if e.Ambiente != origem.AmbienteProducao {
			return nil, fmt.Errorf("%q: só chave de produção entra em chaves-publicas.json (ambiente %q)", e.Kid, e.Ambiente)
		}
		if !formatoDoKid.MatchString(e.Kid) {
			return nil, fmt.Errorf("%q: kid fora do formato", e.Kid)
		}
		if vistos[e.Kid] {
			return nil, fmt.Errorf("%q: kid repetido", e.Kid)
		}
		vistos[e.Kid] = true
		if e.Jwk.Kty != "EC" || e.Jwk.Crv != "P-256" {
			return nil, fmt.Errorf("%q: a JWK não é EC P-256", e.Kid)
		}
		if _, ok := origem.Padroes[e.Iss]; !ok {
			return nil, fmt.Errorf("%q: emissor desconhecido", e.Kid)
		}
		if err := pontoP256(e.Jwk.X, e.Jwk.Y); err != nil {
			return nil, fmt.Errorf("%q: %w", e.Kid, err)
		}
		fmt.Fprintf(&b, "\t{Kid: %q, Emissor: %q, Ambiente: %q, X: %q, Y: %q},\n", e.Kid, e.Iss, e.Ambiente, e.Jwk.X, e.Jwk.Y)
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}

// pontoP256 confere x e y como o programa confere ao carregar (`bilhete.ChaveDeJwk`): base64url
// canônico de 32 bytes e ponto na curva. O gerador não importa o pacote `bilhete`, que depende do
// arquivo que ele gera: um `chaves.go` quebrado não pode impedir que se gere o certo.
func pontoP256(x, y string) error {
	bx, errX := base64.RawURLEncoding.Strict().DecodeString(x)
	by, errY := base64.RawURLEncoding.Strict().DecodeString(y)
	if errX != nil || errY != nil || len(bx) != 32 || len(by) != 32 || !alfabeto.MatchString(x) || !alfabeto.MatchString(y) {
		return errors.New("x e y da JWK precisam ser base64url de 32 bytes")
	}
	if _, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), append(append([]byte{0x04}, bx...), by...)); err != nil {
		return errors.New("ponto fora da curva P-256")
	}
	return nil
}
