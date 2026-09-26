package mensagens

import (
	"regexp"
	"slices"
	"unicode/utf8"

	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// Pedido é um pedido da extensão, decodificado e conferido.
type Pedido struct {
	ID     string
	Op     string
	Origem string
	// Ref, Digest e Bilhete só em `conferir` e `assinar`.
	Ref     string
	Digest  string
	Bilhete string
	// Pin só em `assinar`, e só quando veio. É a ÚNICA cópia decodificada do PIN: quem usa chama
	// `Zerar` logo depois do login, e em todo caminho de saída.
	Pin    []byte
	TemPin bool
}

// Zerar apaga o PIN.
func (p *Pedido) Zerar() {
	if p.Pin != nil {
		Zerar(p.Pin[:cap(p.Pin)])
		p.Pin = nil
	}
}

var (
	formatoDoID     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	formatoDaOrigem = regexp.MustCompile(`^https?://[a-z0-9.-]+(:[0-9]{1,5})?$`)
	formatoHex64    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	alfabetoDoJWS   = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

const tamanhoMaximoDaOrigem = 256

// camposDeDados diz, por operação, os campos de `dados` obrigatórios e os opcionais. Operação fora
// do mapa não tem `dados`.
var camposDeDados = map[string]struct{ obrigatorios, opcionais []string }{
	protocolo.OpConferir: {obrigatorios: []string{"ref", "digest", "bilhete"}},
	protocolo.OpAssinar:  {obrigatorios: []string{"ref", "digest", "bilhete"}, opcionais: []string{"pin"}},
}

var operacoes = map[string]bool{
	protocolo.OpOla: true, protocolo.OpListar: true, protocolo.OpConferir: true, protocolo.OpAssinar: true, protocolo.OpDiagnostico: true,
}

func recusa(detalhe string) *protocolo.Erro {
	return protocolo.Novo(protocolo.Protocolo, detalhe)
}

// Decodificar confere e decodifica o corpo de um pedido. Na recusa, o `Pedido` devolvido leva o
// `ID` quando ele pôde ser lido, para a resposta chegar a quem pediu; o detalhe nunca cita o
// conteúdo do pedido.
func Decodificar(corpo []byte) (Pedido, *protocolo.Erro) {
	raiz, l, err := lerDocumento(corpo)
	if err != nil {
		l.zerarTudo()
		return Pedido{}, recusa("json fora do protocolo")
	}
	var p Pedido
	e := preencher(&p, raiz)
	if e != nil {
		l.zerarTudo()
		p.Pin = nil
		p.TemPin = false
		return Pedido{ID: p.ID}, e
	}
	l.zerarMenos(p.Pin)
	return p, nil
}

func texto(v valor) (string, bool) {
	if v.tipo != tipoTexto {
		return "", false
	}
	return string(v.bytes), true
}

func preencher(p *Pedido, raiz valor) *protocolo.Erro {
	campos := map[string]valor{}
	for _, m := range raiz.membros {
		campos[m.nome] = m.valor
	}
	// O id primeiro: com ele, até a recusa volta a quem pediu.
	if v, ok := campos["id"]; ok {
		if id, ok := texto(v); ok && formatoDoID.MatchString(id) {
			p.ID = id
		}
	}
	for nome := range campos {
		switch nome {
		case "v", "id", "op", "origem", "dados":
		default:
			return recusa("campo desconhecido")
		}
	}
	if v, ok := campos["v"]; !ok || v.tipo != tipoNumero || string(v.bytes) != "1" {
		return recusa("versão do protocolo")
	}
	if p.ID == "" {
		return recusa("id")
	}
	op, ok := texto(campos["op"])
	if !ok || !operacoes[op] {
		return recusa("operação desconhecida")
	}
	p.Op = op
	o, ok := texto(campos["origem"])
	if !ok || len(o) > tamanhoMaximoDaOrigem || !formatoDaOrigem.MatchString(o) {
		return recusa("origem fora do formato")
	}
	p.Origem = o

	dados, temDados := campos["dados"]
	regra, precisaDeDados := camposDeDados[op]
	if !precisaDeDados {
		if temDados {
			return recusa("operação sem dados")
		}
		return nil
	}
	if !temDados || dados.tipo != tipoObjeto {
		return recusa("dados ausentes")
	}
	porNome := map[string]valor{}
	for _, m := range dados.membros {
		if !slices.Contains(regra.obrigatorios, m.nome) && !slices.Contains(regra.opcionais, m.nome) {
			return recusa("campo desconhecido em dados")
		}
		if m.valor.tipo != tipoTexto {
			return recusa("dados com tipo errado")
		}
		porNome[m.nome] = m.valor
	}
	for _, n := range regra.obrigatorios {
		if _, ok := porNome[n]; !ok {
			return recusa("dados incompletos")
		}
	}
	p.Ref = string(porNome["ref"].bytes)
	p.Digest = string(porNome["digest"].bytes)
	p.Bilhete = string(porNome["bilhete"].bytes)
	if !formatoHex64.MatchString(p.Ref) {
		return recusa("ref fora do formato")
	}
	if !formatoHex64.MatchString(p.Digest) {
		return recusa("digest fora do formato")
	}
	// Só a forma: quem confere o bilhete é o pacote `bilhete`, na ordem do contrato.
	if len(p.Bilhete) == 0 || len(p.Bilhete) > protocolo.TamanhoMaximoDoBilhete || !alfabetoDoJWS.MatchString(p.Bilhete) {
		return protocolo.Novo(protocolo.BilheteInvalido, "etapa forma")
	}
	if pin, ok := porNome["pin"]; ok {
		p.Pin = pin.bytes
		p.TemPin = true
		if len(p.Pin) > protocolo.TetoDoPin {
			return recusa("pin acima do teto")
		}
		for b := p.Pin; len(b) > 0; {
			r, n := utf8.DecodeRune(b)
			if r < 0x20 || r == 0x7f {
				return recusa("pin com caractere de controle")
			}
			b = b[n:]
		}
	}
	return nil
}
