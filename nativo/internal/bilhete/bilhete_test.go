package bilhete

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

var pastaDasFixtures = filepath.Join("..", "..", "..", "protocolo", "fixtures", "bilhete")

type chaveDaFixture struct {
	Kid      string `json:"kid"`
	Iss      string `json:"iss"`
	Ambiente string `json:"ambiente"`
	Jwk      struct {
		Kty string `json:"kty"`
		Crv string `json:"crv"`
		X   string `json:"x"`
		Y   string `json:"y"`
	} `json:"jwk"`
}

func chavesDasFixtures(t testing.TB) []Chave {
	t.Helper()
	bruto, err := os.ReadFile(filepath.Join(pastaDasFixtures, "chaves.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lidas []chaveDaFixture
	if err := json.Unmarshal(bruto, &lidas); err != nil {
		t.Fatal(err)
	}
	var chaves []Chave
	for _, l := range lidas {
		c, err := ChaveDeJwk(l.Kid, l.Iss, l.Ambiente, l.Jwk.X, l.Jwk.Y)
		if err != nil {
			t.Fatalf("chave %s: %v", l.Kid, err)
		}
		chaves = append(chaves, c)
	}
	return chaves
}

type casoDaFixture struct {
	Nome      string `json:"nome"`
	Descricao string `json:"descricao"`
	Jws       string `json:"jws"`
	Entrada   struct {
		Origem            string `json:"origem"`
		Digest            string `json:"digest"`
		CertificadoSha256 string `json:"certificadoSha256"`
		Agora             int64  `json:"agora"`
	} `json:"entrada"`
	Esperado struct {
		OK      bool     `json:"ok"`
		Bilhete *Bilhete `json:"bilhete"`
		Chave   *struct {
			Kid      string `json:"kid"`
			Iss      string `json:"iss"`
			Ambiente string `json:"ambiente"`
		} `json:"chave"`
		Codigo string `json:"codigo"`
		Etapa  string `json:"etapa"`
	} `json:"esperado"`
}

func casosDasFixtures(t testing.TB) []casoDaFixture {
	t.Helper()
	arquivos, err := filepath.Glob(filepath.Join(pastaDasFixtures, "casos", "*.json"))
	if err != nil || len(arquivos) == 0 {
		t.Fatalf("sem casos: %v", err)
	}
	var casos []casoDaFixture
	for _, a := range arquivos {
		bruto, err := os.ReadFile(a)
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(strings.NewReader(string(bruto)))
		dec.DisallowUnknownFields()
		var c casoDaFixture
		if err := dec.Decode(&c); err != nil {
			t.Fatalf("%s: %v", a, err)
		}
		casos = append(casos, c)
	}
	return casos
}

// O contrato: cada caso das fixtures, com TODAS as chaves, dá o mesmo resultado que a referência
// em TypeScript, inclusive a ETAPA (que prova a ordem).
func TestConfereTodasAsFixtures(t *testing.T) {
	chaves := chavesDasFixtures(t)
	casos := casosDasFixtures(t)
	if len(casos) < 78 {
		t.Fatalf("esperava ao menos 78 casos, achei %d", len(casos))
	}
	for _, c := range casos {
		t.Run(c.Nome, func(t *testing.T) {
			r := Conferir(c.Jws, chaves, Entrada{
				Origem:            c.Entrada.Origem,
				Digest:            c.Entrada.Digest,
				CertificadoSha256: c.Entrada.CertificadoSha256,
				Agora:             time.Unix(c.Entrada.Agora, 0),
			})
			if r.OK != c.Esperado.OK {
				t.Fatalf("%s\nok = %v (%s/%s), esperado %v", c.Descricao, r.OK, r.Codigo, r.Etapa, c.Esperado.OK)
			}
			if !r.OK {
				if string(r.Codigo) != c.Esperado.Codigo || string(r.Etapa) != c.Esperado.Etapa {
					t.Fatalf("%s\nparou em %s/%s, esperado %s/%s", c.Descricao, r.Codigo, r.Etapa, c.Esperado.Codigo, c.Esperado.Etapa)
				}
				return
			}
			if c.Esperado.Bilhete == nil || r.Bilhete != *c.Esperado.Bilhete {
				t.Fatalf("%s\nbilhete %+v\nesperado %+v", c.Descricao, r.Bilhete, c.Esperado.Bilhete)
			}
			if c.Esperado.Chave == nil || r.Chave.Kid != c.Esperado.Chave.Kid || r.Chave.Emissor != c.Esperado.Chave.Iss || r.Chave.Ambiente != c.Esperado.Chave.Ambiente {
				t.Fatalf("chave %+v, esperado %+v", r.Chave, c.Esperado.Chave)
			}
		})
	}
}

// Sem chave pinada nenhuma, nada passa: é o programa de release antes da F7a.
func TestSemChavesNadaPassa(t *testing.T) {
	for _, c := range casosDasFixtures(t) {
		if !c.Esperado.OK {
			continue
		}
		r := Conferir(c.Jws, nil, Entrada{Origem: c.Entrada.Origem, Digest: c.Entrada.Digest, CertificadoSha256: c.Entrada.CertificadoSha256, Agora: time.Unix(c.Entrada.Agora, 0)})
		if r.OK || r.Etapa != EtapaKid {
			t.Fatalf("%s: sem chaves deu %v %s", c.Nome, r.OK, r.Etapa)
		}
	}
}

// As constantes daqui são as de `protocolo.json`: a biblioteca é a fonte.
func TestConstantesSaoAsDaBiblioteca(t *testing.T) {
	bruto, err := os.ReadFile(filepath.Join(pastaDasFixtures, "protocolo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		ProtocoloDaPagina      int                        `json:"protocoloDaPagina"`
		Tipo                   string                     `json:"tipo"`
		Algoritmo              string                     `json:"algoritmo"`
		Versao                 int                        `json:"versao"`
		ValidadeS              int                        `json:"validadeS"`
		ToleranciaS            int                        `json:"toleranciaS"`
		TamanhoMaximoDoBilhete int                        `json:"tamanhoMaximoDoBilhete"`
		LimiteDoDocumento      int                        `json:"limiteDoDocumento"`
		LimiteDaOrganizacao    int                        `json:"limiteDaOrganizacao"`
		FaixasDeControle       [][2]rune                  `json:"faixasDeControle"`
		FaixasDeEspaco         [][2]rune                  `json:"faixasDeEspaco"`
		FormatosDaCarga        map[string]string          `json:"formatosDaCarga"`
		InteiroMaximo          int64                      `json:"inteiroMaximo"`
		OrdemDaConferencia     []Etapa                    `json:"ordemDaConferencia"`
		CodigoDaEtapa          map[Etapa]protocolo.Codigo `json:"codigoDaEtapa"`
	}
	if err := json.Unmarshal(bruto, &p); err != nil {
		t.Fatal(err)
	}
	igual := func(nome string, a, b any) {
		t.Helper()
		ja, _ := json.Marshal(a)
		jb, _ := json.Marshal(b)
		if string(ja) != string(jb) {
			t.Errorf("%s: aqui %s, na biblioteca %s", nome, ja, jb)
		}
	}
	igual("protocolo", protocolo.Versao, p.ProtocoloDaPagina)
	igual("tipo", Tipo, p.Tipo)
	igual("algoritmo", Algoritmo, p.Algoritmo)
	igual("versao", Versao, p.Versao)
	igual("validade", ValidadeS, p.ValidadeS)
	igual("tolerancia", ToleranciaS, p.ToleranciaS)
	igual("tamanho", TamanhoMaximo, p.TamanhoMaximoDoBilhete)
	igual("limite do documento", LimiteDoDocumento, p.LimiteDoDocumento)
	igual("limite da organizacao", LimiteDaOrganizacao, p.LimiteDaOrganizacao)
	igual("faixas de controle", FaixasDeControle, p.FaixasDeControle)
	igual("faixas de espaco", FaixasDeEspaco, p.FaixasDeEspaco)
	igual("formatos", map[string]string{"aud": FormatoDeAud, "sid": FormatoDeSid, "hex": FormatoHex}, p.FormatosDaCarga)
	igual("inteiro maximo", int64(InteiroMaximo), p.InteiroMaximo)
	igual("ordem", Ordem, p.OrdemDaConferencia)
	igual("codigo da etapa", CodigoDaEtapa, p.CodigoDaEtapa)
}

// Entrada hostil nunca derruba a conferência, e nunca passa sem chave.
func FuzzConferir(f *testing.F) {
	for _, c := range casosDasFixtures(f) {
		f.Add(c.Jws, c.Entrada.Origem)
	}
	chaves := chavesDasFixtures(f)
	f.Fuzz(func(t *testing.T, jws, origem string) {
		r := Conferir(jws, chaves, Entrada{Origem: origem, Digest: strings.Repeat("a", 64), CertificadoSha256: strings.Repeat("b", 64), Agora: time.Unix(1790000005, 0)})
		if r.OK && (r.Bilhete.Dig != strings.Repeat("a", 64) || r.Bilhete.Aud != origem) {
			t.Fatalf("passou com resumo ou origem diferentes: %+v", r.Bilhete)
		}
		if !r.OK && !slices.Contains(Ordem, r.Etapa) {
			t.Fatalf("etapa desconhecida %q", r.Etapa)
		}
	})
}
