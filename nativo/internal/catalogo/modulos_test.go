package catalogo

import (
	"path"
	"regexp"
	"testing"
	"time"
)

func conferirMedicoes(t *testing.T, nome string, medicoes []Medicao) {
	t.Helper()
	if len(medicoes) == 0 {
		t.Errorf("%s: sem medição", nome)
	}
	for _, med := range medicoes {
		if _, err := time.Parse(time.DateOnly, med.Em); err != nil || med.Por == "" || med.Equipamento == "" || med.Resultado == "" || med.Registro == "" {
			t.Errorf("%s: medição incompleta %+v", nome, med)
		}
	}
}

// Toda entrada tem medição completa, e todo caminho é absoluto. Entrada sem medição não entra.
func TestTodaEntradaFoiMedida(t *testing.T) {
	nomes := map[string]bool{}
	nome := regexp.MustCompile(`^[a-z0-9-]{1,32}$`)
	for _, m := range Modulos {
		if !nome.MatchString(m.Nome) || nomes[m.Nome] {
			t.Errorf("nome inválido ou repetido: %q", m.Nome)
		}
		nomes[m.Nome] = true
		if m.Rotulo == "" || m.Fabricante == "" || len(m.Caminhos) == 0 {
			t.Errorf("%s: sem rótulo, sem fabricante ou sem caminho", m.Nome)
		}
		for plataforma, caminhos := range m.Caminhos {
			for _, c := range caminhos {
				// Os caminhos do catálogo são do Linux e do macOS, com `/`: o `filepath` do Windows,
				// onde a suíte também roda, não os tomaria por absolutos (sem letra de unidade).
				if !path.IsAbs(c) {
					t.Errorf("%s/%s: caminho relativo %q", m.Nome, plataforma, c)
				}
			}
		}
		conferirMedicoes(t, m.Nome, m.Medicoes)
	}
	atr := regexp.MustCompile(`^([0-9A-F]{2}){2,33}$`)
	vistos := map[string]bool{}
	for _, a := range ATRs {
		if !atr.MatchString(a.Valor) || vistos[a.Valor] {
			t.Errorf("ATR fora do formato ou repetido: %q", a.Valor)
		}
		vistos[a.Valor] = true
		if !nomes[a.Modulo] || a.Cartao == "" {
			t.Errorf("%s: módulo %q fora do catálogo, ou sem descrição do cartão", a.Valor, a.Modulo)
		}
		conferirMedicoes(t, a.Valor, a.Medicoes)
	}
}

func TestPeloArquivo(t *testing.T) {
	for caminho, quer := range map[string]string{
		"/usr/lib64/libaetpkss.so.3":                         "safesign",
		"/usr/lib/aarch64-linux-gnu/pkcs11/opensc-pkcs11.so": "opensc",
		"/opt/outro/libfabricante.so":                        "",
	} {
		m, ok := PeloArquivo(caminho, Modulos)
		if (quer == "") == ok || (ok && m.Nome != quer) {
			t.Errorf("%s: %q %v", caminho, m.Nome, ok)
		}
	}
}

func TestModuloDoATR(t *testing.T) {
	modulos := []Modulo{{Nome: "fabricante", Rotulo: "Fabricante"}}
	atrs := []ATR{{Valor: "3B0102", Cartao: "cartão de teste", Modulo: "fabricante"}, {Valor: "3B0103", Cartao: "órfão", Modulo: "sumiu"}}
	if m, a, ok := ModuloDoATR("3B0102", atrs, modulos); !ok || m.Rotulo != "Fabricante" || a.Cartao != "cartão de teste" {
		t.Fatalf("%v %v %v", m, a, ok)
	}
	for _, atr := range []string{"3B0103", "3B01", "3b0102", ""} {
		if _, _, ok := ModuloDoATR(atr, atrs, modulos); ok {
			t.Errorf("%q casou", atr)
		}
	}
}
