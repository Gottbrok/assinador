package protocolo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A lista de códigos é a MESMA da biblioteca, pela fixture que ela gera: um código a mais ou a
// menos de um lado quebra este teste.
func TestCodigosSaoOsDaBiblioteca(t *testing.T) {
	bruto, err := os.ReadFile(filepath.Join("..", "..", "..", "protocolo", "fixtures", "bilhete", "protocolo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		CodigosDeErro []string `json:"codigosDeErro"`
	}
	if err := json.Unmarshal(bruto, &fixture); err != nil {
		t.Fatal(err)
	}
	daqui := make([]string, 0, len(Codigos))
	for _, c := range Codigos {
		daqui = append(daqui, string(c))
	}
	slices.Sort(daqui)
	dela := slices.Clone(fixture.CodigosDeErro)
	slices.Sort(dela)
	if !slices.Equal(daqui, dela) {
		t.Fatalf("códigos divergem da biblioteca\naqui: %v\nlá:   %v", daqui, dela)
	}
	if len(slices.Compact(slices.Clone(daqui))) != len(daqui) {
		t.Fatal("código repetido")
	}
}

func TestDetalheDoPinErradoEhObjeto(t *testing.T) {
	r := Falha("x", PinErrado(TentativasUltima))
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"v":1,"id":"x","ok":false,"erro":{"codigo":"pin-incorreto","detalhe":{"tentativas":"ultima"}}}` {
		t.Fatalf("json: %s", b)
	}
	b, _ = json.Marshal(Falha("x", PinErrado("")))
	if string(b) != `{"v":1,"id":"x","ok":false,"erro":{"codigo":"pin-incorreto"}}` {
		t.Fatalf("sem tentativas: %s", b)
	}
	b, _ = json.Marshal(Falha("x", Novo(Interno, "falhou")))
	if string(b) != `{"v":1,"id":"x","ok":false,"erro":{"codigo":"interno","detalhe":"falhou"}}` {
		t.Fatalf("texto: %s", b)
	}
}
