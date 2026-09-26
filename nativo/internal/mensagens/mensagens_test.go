package mensagens

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

const (
	ref    = "955d65f8a337fc5a58b60eac118e5705ed96b726fc1e71327be6b9b8d2b4f14b"
	digest = "bedd1e3cdd5ed81bb585187d0bf3328ed6197d73ecc01e6e51408471a84e7767"
	jws    = "eyJh.eyJ2.c2ln"
	// u é o início de um escape JSON de unidade UTF-16, montado à parte.
	u = "\\" + "u"
)

func TestQuadroIdaEVolta(t *testing.T) {
	var buf bytes.Buffer
	for _, m := range []string{`{"a":1}`, "", strings.Repeat("x", 1000)} {
		if err := EscreverQuadro(&buf, []byte(m), 64*1024); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range []string{`{"a":1}`, "", strings.Repeat("x", 1000)} {
		b, err := LerQuadro(&buf, 64*1024)
		if err != nil || string(b) != m {
			t.Fatalf("%q %v", b, err)
		}
	}
	if _, err := LerQuadro(&buf, 64*1024); !errors.Is(err, io.EOF) {
		t.Fatalf("fim entre mensagens: %v", err)
	}
}

func TestQuadroCortadoEGrande(t *testing.T) {
	var buf bytes.Buffer
	_ = EscreverQuadro(&buf, []byte("abcdef"), 100)
	cortado := buf.Bytes()[:7]
	if _, err := LerQuadro(bytes.NewReader(cortado), 100); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("corpo cortado: %v", err)
	}
	if _, err := LerQuadro(bytes.NewReader([]byte{1, 2}), 100); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("cabeçalho cortado: %v", err)
	}
	buf.Reset()
	_ = EscreverQuadro(&buf, bytes.Repeat([]byte("x"), 101), 1000)
	if _, err := LerQuadro(&buf, 100); !errors.Is(err, ErrQuadroGrande) {
		t.Fatalf("acima do teto: %v", err)
	}
	if err := EscreverQuadro(io.Discard, make([]byte, 11), 10); !errors.Is(err, ErrQuadroGrande) {
		t.Fatalf("escrita acima do teto: %v", err)
	}
}

func pedido(op, dados string) string {
	s := `{"v":1,"id":"3f2b8c1e-7a4d","op":"` + op + `","origem":"https://demot.confidata.app"`
	if dados != "" {
		s += `,"dados":` + dados
	}
	return s + "}"
}

func TestDecodificaOsPedidosDoProtocolo(t *testing.T) {
	p, e := Decodificar([]byte(pedido("ola", "")))
	if e != nil || p.Op != "ola" || p.ID != "3f2b8c1e-7a4d" || p.Origem != "https://demot.confidata.app" {
		t.Fatalf("ola: %+v %v", p, e)
	}
	p, e = Decodificar([]byte(pedido("conferir", `{"ref":"`+ref+`","digest":"`+digest+`","bilhete":"`+jws+`"}`)))
	if e != nil || p.Ref != ref || p.Digest != digest || p.Bilhete != jws || p.TemPin {
		t.Fatalf("conferir: %+v %v", p, e)
	}
	// Os escapes são montados com a barra à parte (`u` é "\\" + "u"), para o texto do arquivo
	// não virar o caractere que o escape representa.
	pinEscapado := "12" + u + "00334" + u + "00e7" + u + "d83d" + u + "de00" + `\"\\`
	p, e = Decodificar([]byte(" \n" + pedido("assinar", `{"ref":"`+ref+`","digest":"`+digest+`","bilhete":"`+jws+`","pin":"`+pinEscapado+`"}`) + "\t"))
	if e != nil || string(p.Pin) != "1234"+string(rune(0xe7))+string(rune(0x1f600))+`"\` || !p.TemPin {
		t.Fatalf("assinar: %q %v", p.Pin, e)
	}
	p.Zerar()
	if p.Pin != nil {
		t.Fatal("Zerar não soltou o PIN")
	}
	p, e = Decodificar([]byte(pedido("assinar", `{"pin":"","bilhete":"`+jws+`","digest":"`+digest+`","ref":"`+ref+`"}`)))
	if e != nil || !p.TemPin || len(p.Pin) != 0 {
		t.Fatalf("PIN vazio é decisão do host, não do leitor: %+v %v", p, e)
	}
}

func TestRecusaOQueNaoEhDoProtocolo(t *testing.T) {
	dadosOk := `{"ref":"` + ref + `","digest":"` + digest + `","bilhete":"` + jws + `"}`
	casos := map[string]string{
		"vazio":                 ``,
		"lista":                 `[]`,
		"nulo":                  `null`,
		"lixo depois":           pedido("ola", "") + ` {}`,
		"chave repetida":        `{"v":1,"id":"a","id":"b","op":"ola","origem":"https://ushield.app"}`,
		"chave maiuscula":       `{"v":1,"ID":"a","op":"ola","origem":"https://ushield.app"}`,
		"campo desconhecido":    `{"v":1,"id":"a","op":"ola","origem":"https://ushield.app","x":"y"}`,
		"v como texto":          `{"v":"1","id":"a","op":"ola","origem":"https://ushield.app"}`,
		"v decimal":             `{"v":1.0,"id":"a","op":"ola","origem":"https://ushield.app"}`,
		"v dois":                `{"v":2,"id":"a","op":"ola","origem":"https://ushield.app"}`,
		"sem v":                 `{"id":"a","op":"ola","origem":"https://ushield.app"}`,
		"sem id":                `{"v":1,"op":"ola","origem":"https://ushield.app"}`,
		"id com espaco":         `{"v":1,"id":"a b","op":"ola","origem":"https://ushield.app"}`,
		"op desconhecida":       `{"v":1,"id":"a","op":"apagar","origem":"https://ushield.app"}`,
		"origem com caminho":    `{"v":1,"id":"a","op":"ola","origem":"https://ushield.app/x"}`,
		"origem maiuscula":      `{"v":1,"id":"a","op":"ola","origem":"https://USHIELD.app"}`,
		"ola com dados":         pedido("ola", `{}`),
		"conferir sem dados":    pedido("conferir", ""),
		"conferir com pin":      pedido("conferir", `{"ref":"`+ref+`","digest":"`+digest+`","bilhete":"`+jws+`","pin":"1"}`),
		"dados incompletos":     pedido("assinar", `{"ref":"`+ref+`","digest":"`+digest+`"}`),
		"dados com numero":      pedido("assinar", `{"ref":"`+ref+`","digest":"`+digest+`","bilhete":"`+jws+`","pin":1234}`),
		"dados com objeto":      pedido("assinar", `{"ref":{}}`),
		"ref maiuscula":         pedido("conferir", strings.Replace(dadosOk, ref, strings.ToUpper(ref), 1)),
		"digest curto":          pedido("conferir", strings.Replace(dadosOk, digest, digest[:62], 1)),
		"verdadeiro":            `{"v":1,"id":"a","op":"ola","origem":true}`,
		"utf8 invalido":         "{\"v\":1,\"id\":\"a\",\"op\":\"ola\",\"origem\":\"https://ushield.app\xff\"}",
		"surrogate solto":       `{"v":1,"id":"a\ud800","op":"ola","origem":"https://ushield.app"}`,
		"surrogate baixo solto": `{"v":1,"id":"a\udc00","op":"ola","origem":"https://ushield.app"}`,
		"controle cru":          "{\"v\":1,\"id\":\"a\x01\",\"op\":\"ola\",\"origem\":\"https://ushield.app\"}",
		"escape desconhecido":   `{"v":1,"id":"a\x41","op":"ola","origem":"https://ushield.app"}`,
		"texto sem fim":         `{"v":1,"id":"a`,
		"virgula sobrando":      `{"v":1,"id":"a","op":"ola","origem":"https://ushield.app",}`,
		"fundo demais":          pedido("assinar", `{"ref":{"a":{}}}`),
		"numero sem digito":     `{"v":-,"id":"a","op":"ola","origem":"https://ushield.app"}`,
		"zero a esquerda":       `{"v":01,"id":"a","op":"ola","origem":"https://ushield.app"}`,
	}
	for nome, entrada := range casos {
		p, e := Decodificar([]byte(entrada))
		if e == nil {
			t.Errorf("%s: aceitou %+v", nome, p)
			continue
		}
		if e.Codigo != protocolo.Protocolo {
			t.Errorf("%s: código %s", nome, e.Codigo)
		}
		if p.Pin != nil || p.TemPin {
			t.Errorf("%s: a recusa devolveu PIN", nome)
		}
	}
}

func TestBilheteForaDaFormaEhBilheteInvalido(t *testing.T) {
	for _, b := range []string{"", "tem espaço", strings.Repeat("a", protocolo.TamanhoMaximoDoBilhete+1), "a\\r\\nb"} {
		_, e := Decodificar([]byte(pedido("conferir", `{"ref":"`+ref+`","digest":"`+digest+`","bilhete":"`+b+`"}`)))
		if e == nil || e.Codigo != protocolo.BilheteInvalido {
			t.Errorf("%q: %v", b, e)
		}
	}
}

func TestPinForaDoTetoOuComControle(t *testing.T) {
	base := `{"ref":"` + ref + `","digest":"` + digest + `","bilhete":"` + jws + `","pin":"%s"}`
	for nome, pin := range map[string]string{
		"acima do teto": strings.Repeat("1", protocolo.TetoDoPin+1),
		"nul":           `12\u00004`,
		"quebra":        `12\n4`,
		"del":           "12\x7f4",
	} {
		p, e := Decodificar([]byte(pedido("assinar", strings.Replace(base, "%s", pin, 1))))
		if e == nil || e.Codigo != protocolo.Protocolo || p.Pin != nil {
			t.Errorf("%s: %+v %v", nome, p, e)
		}
	}
	p, e := Decodificar([]byte(pedido("assinar", strings.Replace(base, "%s", strings.Repeat("1", protocolo.TetoDoPin), 1))))
	if e != nil || len(p.Pin) != protocolo.TetoDoPin {
		t.Fatalf("no teto: %v", e)
	}
	p.Zerar()
}

// Depois de decodificar, o ÚNICO buffer com o PIN é o do pedido; os outros textos decodificados
// foram zerados. Depois de `Zerar`, nem ele.
func TestSoOPedidoGuardaOPin(t *testing.T) {
	corpo := []byte(pedido("assinar", `{"ref":"`+ref+`","digest":"`+digest+`","bilhete":"`+jws+`","pin":"987654"}`))
	raiz, l, err := lerDocumento(corpo)
	if err != nil {
		t.Fatal(err)
	}
	textos := append([][]byte(nil), l.textos...)
	var p Pedido
	if e := preencher(&p, raiz); e != nil {
		t.Fatal(e)
	}
	l.zerarMenos(p.Pin)
	pinVisto := false
	for _, tx := range textos {
		inteiro := tx[:cap(tx)]
		if len(inteiro) > 0 && &inteiro[0] == &p.Pin[:cap(p.Pin)][0] {
			pinVisto = true
			continue
		}
		if !bytes.Equal(inteiro, make([]byte, len(inteiro))) {
			t.Errorf("texto não zerado: %q", inteiro)
		}
	}
	if !pinVisto || string(p.Pin) != "987654" {
		t.Fatalf("PIN: %q visto=%v", p.Pin, pinVisto)
	}
	arranjo := p.Pin[:cap(p.Pin)]
	p.Zerar()
	if !bytes.Equal(arranjo, make([]byte, len(arranjo))) {
		t.Fatal("Zerar deixou o PIN no buffer")
	}
}

// O que o leitor aceita, o `encoding/json/v2` (que recusa chave repetida e UTF-8 inválido por
// padrão) também aceita, com os MESMOS valores. O leitor pode ser mais estrito, nunca mais frouxo.
func FuzzLeitorConcordaComOJsonPadrao(f *testing.F) {
	f.Add([]byte(pedido("assinar", `{"ref":"`+ref+`","digest":"`+digest+`","bilhete":"`+jws+`","pin":"12`+u+`00e734`+u+`d83d`+u+`de00"}`)))
	f.Add([]byte(`{"a":"` + u + `d800"}`))
	f.Add([]byte(`{"a":"\/\b\f\n\r\t","b":-0.5e+3,"c":{}}`))
	f.Add([]byte(`{"a":"x","a":"y"}`))
	f.Fuzz(func(t *testing.T, entrada []byte) {
		v, l, err := lerDocumento(entrada)
		defer l.zerarTudo()
		if err != nil {
			return
		}
		var padrao any
		if err := jsonv2.Unmarshal(entrada, &padrao); err != nil {
			t.Fatalf("o leitor aceitou e o json/v2 recusou (%v): %q", err, entrada)
		}
		if !mesmoValor(v, padrao) {
			t.Fatalf("valores divergem: %q", entrada)
		}
	})
}

func mesmoValor(v valor, padrao any) bool {
	switch v.tipo {
	case tipoTexto:
		s, ok := padrao.(string)
		return ok && s == string(v.bytes)
	case tipoNumero:
		_, ok := padrao.(float64)
		return ok
	case tipoObjeto:
		m, ok := padrao.(map[string]any)
		if !ok || len(m) != len(v.membros) {
			return false
		}
		for _, mb := range v.membros {
			if !mesmoValor(mb.valor, m[mb.nome]) {
				return false
			}
		}
		return true
	}
	return false
}
