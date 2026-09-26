package pkcs11

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Gottbrok/assinador/nativo/internal/catalogo"
)

func arquivo(t *testing.T, caminho, conteudo string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(caminho), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
	return caminho
}

func TestDescobrirNaOrdemESemRepetir(t *testing.T) {
	dir := t.TempDir()
	plataforma := runtime.GOOS + "/" + runtime.GOARCH
	fabricante := arquivo(t, filepath.Join(dir, "lib", "fabricante.so"), "x")
	generico := arquivo(t, filepath.Join(dir, "lib", "generico.so"), "x")
	doSistema := arquivo(t, filepath.Join(dir, "lib", "sistema.so"), "x")
	daPessoa := arquivo(t, filepath.Join(dir, "lib", "pessoa.so"), "x")
	atalho := filepath.Join(dir, "lib", "atalho.so")
	if err := os.Symlink(fabricante, atalho); err != nil {
		t.Fatal(err)
	}
	cat := []catalogo.Modulo{
		{Nome: "fabricante", Rotulo: "Fabricante", Caminhos: map[string][]string{plataforma: {filepath.Join(dir, "nao-existe.so"), fabricante}}},
		{Nome: "generico", Rotulo: "Genérico", Generico: true, Caminhos: map[string][]string{plataforma: {generico}}},
		{Nome: "ausente", Rotulo: "Ausente", Caminhos: map[string][]string{plataforma: {filepath.Join(dir, "ausente.so")}}},
		{Nome: "outra-plataforma", Rotulo: "Outra", Caminhos: map[string][]string{"plan9/386": {fabricante}}},
	}
	sistema := filepath.Join(dir, "etc", "modulos.d")
	arquivo(t, filepath.Join(sistema, "b.conf"), "# comentário\n\n"+doSistema+"\n")
	arquivo(t, filepath.Join(sistema, "a.conf"), atalho+"\nrelativo/nao.so\n")
	arquivo(t, filepath.Join(sistema, "ignorado.txt"), daPessoa+"\n")
	usuario := arquivo(t, filepath.Join(dir, "home", "modulos"), "  "+daPessoa+"  \n"+filepath.Join(dir, "sumiu.so")+"\n"+generico+"\n")

	achados, ausentes := Descobrir(OpcoesDeDescoberta{Catalogo: cat, PastaDoSistema: sistema, ArquivoDoUsuario: usuario})
	var caminhos []string
	for _, m := range achados {
		caminhos = append(caminhos, m.Caminho)
	}
	quer := []string{fabricante, generico, doSistema, daPessoa}
	if len(caminhos) != len(quer) {
		t.Fatalf("achados %v", caminhos)
	}
	for i := range quer {
		if caminhos[i] != quer[i] {
			t.Fatalf("achados %v, esperado %v", caminhos, quer)
		}
	}
	if achados[0].Origem != OrigemCatalogo || !achados[1].Generico || achados[2].Origem != OrigemConfiguracao || achados[3].Rotulo != "pessoa.so" {
		t.Fatalf("metadados: %+v", achados)
	}
	var nomesAusentes []string
	for _, m := range ausentes {
		nomesAusentes = append(nomesAusentes, m.Nome)
	}
	if len(ausentes) != 2 || nomesAusentes[0] != "ausente" || nomesAusentes[1] != "sumiu.so" {
		t.Fatalf("ausentes: %v", nomesAusentes)
	}
}

func TestOpcoesPadraoUsamOCatalogoMedido(t *testing.T) {
	o := OpcoesPadrao()
	if len(o.Catalogo) != len(catalogo.Modulos) || o.PastaDoSistema != "/etc/confidata-assinador/modulos.d" {
		t.Fatalf("%+v", o)
	}
	t.Setenv("XDG_CONFIG_HOME", "/tmp/cfg")
	if o := OpcoesPadrao(); o.ArquivoDoUsuario != "/tmp/cfg/confidata-assinador/modulos" {
		t.Fatalf("arquivo da pessoa: %s", o.ArquivoDoUsuario)
	}
}
