//go:build windows

package pcsc

import "testing"

// No executor do CI não há leitora: o Windows responde que o serviço Cartão Inteligente não está
// rodando (ele só roda com leitora conectada) ou que não há leitora, e nunca inventa uma. Numa
// máquina com leitora, a consulta tem de voltar `ok`, com o nome de cada uma.
func TestConsultarNoWindows(t *testing.T) {
	r := Consultar()
	t.Logf("estado %s, detalhe %q, %d leitora(s)", r.Estado, r.Detalhe, len(r.Leitoras))
	switch r.Estado {
	case EstadoSemServico, EstadoSemLeitora:
		if len(r.Leitoras) != 0 {
			t.Fatalf("%+v", r)
		}
	case EstadoOk:
		for _, l := range r.Leitoras {
			if l.Nome == "" || (!l.ComCartao && l.ATR != "") {
				t.Fatalf("%+v", l)
			}
		}
	default:
		t.Fatalf("%+v", r)
	}
}
