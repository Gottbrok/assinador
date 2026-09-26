//go:build dev

package bilhete

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Gottbrok/assinador/nativo/internal/origem"
)

// ArquivoDeChavesDev é onde o desenvolvedor põe a chave PÚBLICA gerada no console LOCAL (do
// Confidata ou do ushield) ou pela ferramenta `host-teste`. Nenhuma chave privada de
// desenvolvimento fica no repositório.
const ArquivoDeChavesDev = "confidata-assinador/chaves-dev.json"

// jwksDeTeste são as chaves de TESTE das fixtures (`protocolo/fixtures/bilhete/chaves.json`),
// cujas privadas qualquer um recalcula a partir de semente pública. O arquivo de desenvolvimento
// tem a MESMA forma daquele: copiá-lo para cá faria o programa aceitar bilhete forjado por
// qualquer pessoa, para `localhost` e para `https://*.confidata.app`. Por isso toda entrada com
// uma destas coordenadas, ou com kid que comece por `teste`, é recusada (LEIAME das fixtures).
// `dev_test.go` confere esta lista contra a fixture.
var jwksDeTeste = []string{
	"eRreN-j9xszMP0m6HAXD-AqkXYMOiDVQk4wlqgyRdJE",
	"7QUIMJHnjw74prpGn9Ue7rH6-PQfjpA5HkNIcwQ-QrQ",
	"da0-9jsJFcKqQjeo8S8rA1Jg2MeXV6VJBMt_KfQUI4w",
	"D7j5_eCVRPIdeVwt6osZ33bZVFKflhP_9DnmSAOwWTU",
	"qBX8vzzAkN1vpcEHFfjUpu-T80-gTx-NqmqHswaT1nE",
	"Aq4xvoa3hTWKqY2MkTrRHnVJs8YB8q684jKa6Oh8_80",
}

type entradaDeChave struct {
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

func chavesDeDesenvolvimento(pinadas []Chave) ([]Chave, []string) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, []string{"chaves dev: sem diretório de configuração"}
	}
	return lerChavesDev(filepath.Join(dir, ArquivoDeChavesDev), pinadas)
}

func lerChavesDev(caminho string, pinadas []Chave) ([]Chave, []string) {
	f, err := os.Open(caminho)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []string{"chaves dev: " + err.Error()}
	}
	defer f.Close()
	bruto, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil || len(bruto) > 64*1024 {
		return nil, []string{"chaves dev: arquivo ilegível ou acima de 64 KiB"}
	}
	dec := json.NewDecoder(strings.NewReader(string(bruto)))
	dec.DisallowUnknownFields()
	var entradas []entradaDeChave
	if err := dec.Decode(&entradas); err != nil || dec.More() {
		return nil, []string{"chaves dev: o arquivo não é uma lista de chaves"}
	}
	var chaves []Chave
	var avisos []string
	recusar := func(kid, motivo string) {
		avisos = append(avisos, fmt.Sprintf("chaves dev: %q recusada: %s", kid, motivo))
	}
	for _, e := range entradas {
		switch {
		case e.Ambiente != origem.AmbienteDev:
			recusar(e.Kid, "o arquivo local só aceita chave de ambiente dev")
		case strings.HasPrefix(e.Kid, "teste"):
			recusar(e.Kid, "kid de teste (as privadas das chaves de teste são públicas)")
		case slices.Contains(jwksDeTeste, e.Jwk.X) || slices.Contains(jwksDeTeste, e.Jwk.Y):
			recusar(e.Kid, "é uma chave de teste das fixtures (a privada é pública)")
		case e.Jwk.Kty != "EC" || e.Jwk.Crv != "P-256":
			recusar(e.Kid, "a JWK não é EC P-256")
		case slices.ContainsFunc(append(slices.Clone(pinadas), chaves...), func(c Chave) bool { return c.Kid == e.Kid }):
			recusar(e.Kid, "kid repetido")
		default:
			c, err := ChaveDeJwk(e.Kid, e.Iss, e.Ambiente, e.Jwk.X, e.Jwk.Y)
			if err != nil {
				recusar(e.Kid, err.Error())
				continue
			}
			chaves = append(chaves, c)
		}
	}
	return chaves, avisos
}
