package catalogo

import (
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// Toda entrada tem medição completa, e todo caminho é absoluto. Entrada sem medição não entra.
func TestTodaEntradaFoiMedida(t *testing.T) {
	nomes := map[string]bool{}
	nome := regexp.MustCompile(`^[a-z0-9-]{1,32}$`)
	for _, m := range Modulos {
		if !nome.MatchString(m.Nome) || nomes[m.Nome] {
			t.Errorf("nome inválido ou repetido: %q", m.Nome)
		}
		nomes[m.Nome] = true
		if m.Rotulo == "" || len(m.Caminhos) == 0 {
			t.Errorf("%s: sem rótulo ou sem caminho", m.Nome)
		}
		for plataforma, caminhos := range m.Caminhos {
			for _, c := range caminhos {
				if !filepath.IsAbs(c) {
					t.Errorf("%s/%s: caminho relativo %q", m.Nome, plataforma, c)
				}
			}
		}
		if len(m.Medicoes) == 0 {
			t.Errorf("%s: sem medição", m.Nome)
		}
		for _, med := range m.Medicoes {
			if _, err := time.Parse(time.DateOnly, med.Em); err != nil || med.Por == "" || med.Equipamento == "" || med.Resultado == "" || med.Registro == "" {
				t.Errorf("%s: medição incompleta %+v", m.Nome, med)
			}
		}
	}
}
