package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `chaves.go` é exatamente o que o gerador produz do JSON versionado: editar um sem o outro
// reprova aqui.
func TestChavesGeradasEstaoEmDia(t *testing.T) {
	bruto, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "protocolo", "chaves-publicas.json"))
	if err != nil {
		t.Fatal(err)
	}
	quer, err := gerar(bruto)
	if err != nil {
		t.Fatalf("o JSON versionado não gera: %v", err)
	}
	tem, err := os.ReadFile(filepath.Join("..", "chaves.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(quer, tem) {
		t.Fatal("internal/bilhete/chaves.go está desatualizado: rode `go generate ./internal/bilhete`")
	}
}

// As chaves de TESTE das fixtures nunca entram no JSON que vai para o release.
func TestGeradorRecusaChaveDeTesteEDev(t *testing.T) {
	bruto, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "protocolo", "fixtures", "bilhete", "chaves.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gerar(bruto); err == nil || !strings.Contains(err.Error(), "só chave de produção") {
		t.Fatalf("aceitou as chaves de teste: %v", err)
	}
	dev := `[{"kid":"dev-local","iss":"confidata","ambiente":"dev","jwk":{"kty":"EC","crv":"P-256","x":"qBX8vzzAkN1vpcEHFfjUpu-T80-gTx-NqmqHswaT1nE","y":"Aq4xvoa3hTWKqY2MkTrRHnVJs8YB8q684jKa6Oh8_80"}}]`
	if _, err := gerar([]byte(dev)); err == nil {
		t.Fatal("aceitou chave dev")
	}
}

func TestGeradorConfereCadaEntrada(t *testing.T) {
	x, y := "qBX8vzzAkN1vpcEHFfjUpu-T80-gTx-NqmqHswaT1nE", "Aq4xvoa3hTWKqY2MkTrRHnVJs8YB8q684jKa6Oh8_80"
	jwk := func(kid, iss, xx, yy string) string {
		return `{"kid":"` + kid + `","iss":"` + iss + `","ambiente":"producao","jwk":{"kty":"EC","crv":"P-256","x":"` + xx + `","y":"` + yy + `"}}`
	}
	casos := map[string]string{
		"kid repetido":     "[" + jwk("a", "ushield", x, y) + "," + jwk("a", "ushield", x, y) + "]",
		"kid maiusculo":    "[" + jwk("A", "ushield", x, y) + "]",
		"emissor estranho": "[" + jwk("a", "outro", x, y) + "]",
		"ponto fora":       "[" + jwk("a", "ushield", x, x) + "]",
		"campo a mais":     `[{"kid":"a","iss":"ushield","ambiente":"producao","d":"x","jwk":{"kty":"EC","crv":"P-256","x":"` + x + `","y":"` + y + `"}}]`,
		"nao eh lista":     `{}`,
		"lixo depois":      "[] []",
		"curva errada":     `[{"kid":"a","iss":"ushield","ambiente":"producao","jwk":{"kty":"EC","crv":"P-384","x":"` + x + `","y":"` + y + `"}}]`,
	}
	for nome, entrada := range casos {
		if _, err := gerar([]byte(entrada)); err == nil {
			t.Errorf("%s: aceitou", nome)
		}
	}
	codigo, err := gerar([]byte("[" + jwk("ushield-2026-1", "ushield", x, y) + "]"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(codigo), `{Kid: "ushield-2026-1", Emissor: "ushield", Ambiente: "producao"`) {
		t.Fatalf("saída inesperada:\n%s", codigo)
	}
}
