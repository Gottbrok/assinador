//go:build linux

package pcsc

import (
	"encoding/json"
	"os"
	"os/exec"
	"slices"
	"testing"
)

// O binário de teste faz o papel do programa numa consulta de verdade, em processo próprio: o
// pcsc-lite guarda o nome do socket do `pcscd` na primeira conexão do processo, então cada cenário
// precisa de um processo novo.
func TestMain(m *testing.M) {
	if os.Getenv("ASSINADOR_TESTE_PCSC") == "1" {
		_ = json.NewEncoder(os.Stdout).Encode(Consultar())
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func consultarEmProcesso(t *testing.T, ambiente ...string) Resultado {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^$")
	cmd.Env = append(append(os.Environ(), "ASSINADOR_TESTE_PCSC=1"), ambiente...)
	saida, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var r Resultado
	if err := json.Unmarshal(saida, &r); err != nil {
		t.Fatalf("%v: %s", err, saida)
	}
	return r
}

// Com o `pcscd` fora do ar (o socket apontado para um caminho que não existe), a consulta diz
// `sem-servico`, que o diagnóstico transforma em frase. É o cenário do "pcscd parado".
func TestPcscdParadoEhSemServico(t *testing.T) {
	if _, err := os.Stat("/usr/lib/x86_64-linux-gnu/libpcsclite.so.1"); err != nil {
		if _, err2 := os.Stat("/usr/lib64/libpcsclite.so.1"); err2 != nil {
			if _, err3 := os.Stat("/usr/lib/aarch64-linux-gnu/libpcsclite.so.1"); err3 != nil {
				t.Skip("sem o pcsc-lite nesta máquina")
			}
		}
	}
	r := consultarEmProcesso(t, "PCSCLITE_CSOCK_NAME=/nao/existe/pcscd.comm")
	if r.Estado != EstadoSemServico || r.Detalhe != "0x8010001D" || len(r.Leitoras) != 0 {
		t.Fatalf("%+v", r)
	}
}

// A consulta de verdade, nesta máquina, cai num estado conhecido e nunca derruba o processo.
func TestConsultaDeVerdade(t *testing.T) {
	r := consultarEmProcesso(t)
	if !slices.Contains([]string{EstadoOk, EstadoSemBiblioteca, EstadoSemServico, EstadoSemLeitora}, r.Estado) {
		t.Fatalf("estado inesperado: %+v", r)
	}
	t.Logf("nesta máquina: %+v", r)
}

func TestBibliotecaAusente(t *testing.T) {
	anterior := biblioteca
	biblioteca = "libassinador-nao-existe.so.9"
	defer func() { biblioteca = anterior }()
	if r := Consultar(); r.Estado != EstadoSemBiblioteca {
		t.Fatalf("%+v", r)
	}
}

func TestEstadoDoCodigoENomes(t *testing.T) {
	casos := map[uint32]string{codigoSemServico: EstadoSemServico, codigoServicoParou: EstadoSemServico, codigoSemLeitora: EstadoSemLeitora, 0x80100001: EstadoFalhou}
	for c, quer := range casos {
		if got := estadoDoCodigo(c); got != quer {
			t.Errorf("%x: %s", c, got)
		}
	}
	nomes := nomesDaLista([]byte("Leitora A 00 00\x00Leitora B 01 00\x00\x00"))
	if !slices.Equal(nomes, []string{"Leitora A 00 00", "Leitora B 01 00"}) {
		t.Fatalf("%q", nomes)
	}
	longo := make([]byte, tamanhoMaximoDoNome+1)
	for i := range longo {
		longo[i] = 'x'
	}
	if n := nomesDaLista(append(longo, 0)); len(n) != 0 {
		t.Fatalf("aceitou nome acima do teto: %d", len(n))
	}
}
