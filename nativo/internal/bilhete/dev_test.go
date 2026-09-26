//go:build dev

package bilhete

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A lista de coordenadas proibidas é a das fixtures: chave de teste nova na biblioteca sem entrar
// aqui reprova.
func TestListaDeChavesDeTesteBateComAFixture(t *testing.T) {
	bruto, err := os.ReadFile(filepath.Join(pastaDasFixtures, "chaves.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lidas []chaveDaFixture
	if err := json.Unmarshal(bruto, &lidas); err != nil {
		t.Fatal(err)
	}
	var daFixture []string
	for _, l := range lidas {
		daFixture = append(daFixture, l.Jwk.X, l.Jwk.Y)
	}
	slices.Sort(daFixture)
	daqui := slices.Sorted(slices.Values(jwksDeTeste))
	if !slices.Equal(daqui, daFixture) {
		t.Fatalf("coordenadas proibidas divergem da fixture\naqui: %v\nlá:   %v", daqui, daFixture)
	}
}

func jwkDeTeste(t *testing.T) (string, string) {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ponto, err := k.PublicKey.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(ponto[1:33]), base64.RawURLEncoding.EncodeToString(ponto[33:])
}

// O arquivo local recusa, com aviso, tudo o que faria o programa de desenvolvimento aceitar
// bilhete forjado: as chaves das fixtures (a privada é pública), kid de teste, ambiente que não é
// dev. Aceita a chave dev de verdade.
func TestArquivoDeChavesDevRecusaChaveDeTeste(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "chaves-dev.json")

	// A fixture copiada inteira: nenhuma entra.
	fixture, err := os.ReadFile(filepath.Join(pastaDasFixtures, "chaves.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	chaves, avisos := lerChavesDev(caminho, nil)
	if len(chaves) != 0 || len(avisos) != 3 {
		t.Fatalf("fixture copiada: %d chaves, avisos %v", len(chaves), avisos)
	}

	x, y := jwkDeTeste(t)
	xt := jwksDeTeste[4]
	entrada := `[
	{"kid":"dev-local","iss":"confidata","ambiente":"dev","jwk":{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + y + `"}},
	{"kid":"teste-disfarcada","iss":"confidata","ambiente":"dev","jwk":{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + y + `"}},
	{"kid":"outra","iss":"confidata","ambiente":"dev","jwk":{"kty":"EC","crv":"P-256","x":"` + xt + `","y":"` + y + `"}},
	{"kid":"prod-local","iss":"confidata","ambiente":"producao","jwk":{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + y + `"}},
	{"kid":"dev-local","iss":"confidata","ambiente":"dev","jwk":{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + y + `"}}
]`
	if err := os.WriteFile(caminho, []byte(entrada), 0o600); err != nil {
		t.Fatal(err)
	}
	chaves, avisos = lerChavesDev(caminho, nil)
	if len(chaves) != 1 || chaves[0].Kid != "dev-local" || chaves[0].Ambiente != "dev" {
		t.Fatalf("chaves: %+v", chaves)
	}
	if len(avisos) != 4 {
		t.Fatalf("avisos: %v", avisos)
	}

	// Kid igual ao de uma chave pinada de produção também é recusado.
	pinada := Chave{Kid: "dev-local"}
	chaves, _ = lerChavesDev(caminho, []Chave{pinada})
	if slices.ContainsFunc(chaves, func(c Chave) bool { return c.Kid == "dev-local" }) {
		t.Fatal("aceitou kid repetido de chave pinada")
	}

	// Arquivo ausente não é erro; arquivo quebrado é aviso, sem chave.
	if c, a := lerChavesDev(filepath.Join(dir, "nao-existe.json"), nil); c != nil || a != nil {
		t.Fatalf("ausente: %v %v", c, a)
	}
	if err := os.WriteFile(caminho, []byte(`{"x":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if c, a := lerChavesDev(caminho, nil); c != nil || len(a) != 1 {
		t.Fatalf("quebrado: %v %v", c, a)
	}
}
