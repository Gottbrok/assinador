package origem

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Os padrões são os da biblioteca, pela fixture que ela gera.
func TestPadroesSaoOsDaBiblioteca(t *testing.T) {
	bruto, err := os.ReadFile(filepath.Join("..", "..", "..", "protocolo", "fixtures", "bilhete", "protocolo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Emissores          []string            `json:"emissores"`
		PadroesDeOrigem    map[string][]string `json:"padroesDeOrigem"`
		PadroesDeOrigemDev []string            `json:"padroesDeOrigemDev"`
	}
	if err := json.Unmarshal(bruto, &fixture); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(Emissores, fixture.Emissores) {
		t.Fatalf("emissores: aqui %v, lá %v", Emissores, fixture.Emissores)
	}
	if !maps.EqualFunc(Padroes, fixture.PadroesDeOrigem, slices.Equal) {
		t.Fatalf("padrões: aqui %v, lá %v", Padroes, fixture.PadroesDeOrigem)
	}
	if !slices.Equal(PadroesDev, fixture.PadroesDeOrigemDev) {
		t.Fatalf("padrões dev: aqui %v, lá %v", PadroesDev, fixture.PadroesDeOrigemDev)
	}
}

func TestAceita(t *testing.T) {
	casos := []struct {
		origem, emissor, ambiente string
		quer                      bool
	}{
		{"https://demot.confidata.app", Confidata, AmbienteProducao, true},
		{"https://demot.confidata.app", Ushield, AmbienteProducao, false},
		{"https://ushield.app", Ushield, AmbienteProducao, true},
		{"https://www.ushield.app", Ushield, AmbienteProducao, false},
		{"https://demot.confidata.app.evil.com", Confidata, AmbienteProducao, false},
		{"https://demot.confidata.app/", Confidata, AmbienteProducao, false},
		{"https://a.b.confidata.app", Confidata, AmbienteProducao, false},
		{"http://demot.confidata.app", Confidata, AmbienteProducao, false},
		{"https://DEMOT.confidata.app", Confidata, AmbienteProducao, false},
		{"http://localhost:3000", Confidata, AmbienteProducao, false},
		{"http://localhost:3000", Confidata, AmbienteTeste, false},
		{"http://localhost:3000", Confidata, AmbienteDev, true},
		{"http://demot.localhost:3000", Confidata, AmbienteDev, true},
		{"https://demot.confidata.app", Confidata, AmbienteDev, true},
		{"http://localhost:3000", "outro", AmbienteDev, false},
		{"https://ushield.app\n", Ushield, AmbienteProducao, false},
	}
	for _, c := range casos {
		if got := Aceita(c.origem, c.emissor, c.ambiente); got != c.quer {
			t.Errorf("Aceita(%q, %s, %s) = %v", c.origem, c.emissor, c.ambiente, got)
		}
	}
}

func TestPermitidaSemBilhete(t *testing.T) {
	for _, o := range []string{"https://demot.confidata.app", "https://ushield.app"} {
		if !PermitidaSemBilhete(o) {
			t.Errorf("recusou %s", o)
		}
	}
	for _, o := range []string{"https://evil.com", "", "https://ushield.app.evil.com"} {
		if PermitidaSemBilhete(o) {
			t.Errorf("aceitou %s", o)
		}
	}
	if got := PermitidaSemBilhete("http://localhost:3000"); got != localhostSemBilhete {
		t.Errorf("localhost sem bilhete = %v, o build diz %v", got, localhostSemBilhete)
	}
	// A origem das páginas da extensão não é de emissor nenhum: nem lista sem bilhete, nem casa
	// bilhete de ambiente algum. Quem a admite, só para o diagnóstico, é o host.
	if PermitidaSemBilhete(DaExtensao) {
		t.Errorf("a origem da extensão passou como origem de emissor")
	}
	for emissor := range Padroes {
		for _, ambiente := range []string{AmbienteProducao, AmbienteDev, AmbienteTeste} {
			if Aceita(DaExtensao, emissor, ambiente) {
				t.Errorf("a origem da extensão casou o emissor %s (%s)", emissor, ambiente)
			}
		}
	}
}

func TestLerChamador(t *testing.T) {
	id := "abcdefghijklmnopabcdefghijklmnop"
	casos := []struct {
		nome string
		args []string
		ok   bool
		quer Chamador
	}{
		{"chrome no linux", []string{"chrome-extension://" + id + "/"}, true, Chamador{Navegador: Chromium, Extensao: id}},
		{"chrome no windows", []string{"chrome-extension://" + id + "/", "--parent-window=4242"}, true, Chamador{Navegador: Chromium, Extensao: id, JanelaMae: 4242}},
		{"firefox", []string{"/usr/lib/mozilla/native-messaging-hosts/br.com.confidata.assinador.json", ExtensaoFirefox}, true, Chamador{Navegador: Firefox, Extensao: ExtensaoFirefox}},
		{"sem argumentos", nil, false, Chamador{}},
		{"id fora do alfabeto", []string{"chrome-extension://" + "zbcdefghijklmnopabcdefghijklmnop" + "/"}, false, Chamador{}},
		{"id curto", []string{"chrome-extension://abc/"}, false, Chamador{}},
		{"sem a barra final", []string{"chrome-extension://" + id}, false, Chamador{}},
		{"com caminho", []string{"chrome-extension://" + id + "/x"}, false, Chamador{}},
		{"janela repetida", []string{"chrome-extension://" + id + "/", "--parent-window=1", "--parent-window=2"}, false, Chamador{}},
		{"janela nao numerica", []string{"chrome-extension://" + id + "/", "--parent-window=x"}, false, Chamador{}},
		{"argumento a mais", []string{"chrome-extension://" + id + "/", "--outro"}, false, Chamador{}},
		{"firefox sem id", []string{"/caminho.json", ""}, false, Chamador{}},
		{"tres argumentos", []string{"a", "b", "c"}, false, Chamador{}},
	}
	for _, c := range casos {
		got, ok := LerChamador(c.args)
		if ok != c.ok || got != c.quer {
			t.Errorf("%s: %+v %v", c.nome, got, ok)
		}
	}
}

func TestPermitido(t *testing.T) {
	if !(Chamador{Navegador: Firefox, Extensao: ExtensaoFirefox}).Permitido() {
		t.Error("recusou a extensão do Firefox")
	}
	if (Chamador{Navegador: Firefox, Extensao: "outra@exemplo.com"}).Permitido() {
		t.Error("aceitou outra extensão do Firefox")
	}
	// Extensão do Chrome fora da lista deste build não passa, e a lista é a que vai aos manifestos.
	if (Chamador{Navegador: Chromium, Extensao: "abcdefghijklmnopabcdefghijklmnop"}).Permitido() {
		t.Error("aceitou extensão do Chrome sem ID conhecido")
	}
	for _, id := range ExtensoesChrome() {
		if !(Chamador{Navegador: Chromium, Extensao: id}).Permitido() {
			t.Errorf("a lista dos manifestos tem %s, e o programa o recusa", id)
		}
	}
	if (Chamador{}).Permitido() {
		t.Error("aceitou chamador vazio")
	}
}
