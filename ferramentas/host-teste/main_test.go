package main

import (
	"encoding/json"
	"testing"
)

// O pedido com PIN é o `assinar` do protocolo, com o PIN dentro de `dados`.
func TestCorpoComPin(t *testing.T) {
	dados := map[string]string{"ref": "r", "digest": "d", "bilhete": "b"}
	corpo := corpoComPin("host-teste-1", "http://localhost:3000", dados, []byte(`12"34`))
	var m struct {
		V      int               `json:"v"`
		ID     string            `json:"id"`
		Op     string            `json:"op"`
		Origem string            `json:"origem"`
		Dados  map[string]string `json:"dados"`
	}
	if err := json.Unmarshal(corpo, &m); err != nil {
		t.Fatalf("%v: %s", err, corpo)
	}
	if m.V != 1 || m.ID != "host-teste-1" || m.Op != "assinar" || m.Origem != "http://localhost:3000" || m.Dados["pin"] != `12"34` || m.Dados["ref"] != "r" || len(m.Dados) != 4 {
		t.Fatalf("%+v", m)
	}
}

// O PIN escapado à mão é JSON válido e volta igual pelo decodificador padrão.
func TestEscaparJSON(t *testing.T) {
	for _, pin := range []string{"123456", `12"34`, `a\b`, "12\x0134", "çãé"} {
		corpo := append([]byte(`{"pin":"`), escaparJSON(nil, []byte(pin))...)
		corpo = append(corpo, `"}`...)
		var m map[string]string
		if err := json.Unmarshal(corpo, &m); err != nil || m["pin"] != pin {
			t.Errorf("%q: %v %q", pin, err, m["pin"])
		}
	}
}
