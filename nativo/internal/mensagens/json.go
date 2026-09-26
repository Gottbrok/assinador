package mensagens

import (
	"errors"
	"unicode/utf8"
)

// O leitor aceita só o subconjunto de JSON que o protocolo usa: objeto no topo, cujos valores são
// texto, número ou UM nível de objeto (o `dados`), cujos valores são texto. Lista, `true`,
// `false`, `null` e objeto mais fundo são recusados: nenhum pedido os tem.

type tipoDeValor byte

const (
	tipoTexto tipoDeValor = iota + 1
	tipoNumero
	tipoObjeto
)

type valor struct {
	tipo tipoDeValor
	// bytes é o texto decodificado (tipoTexto) ou o literal do número (tipoNumero). O texto mora
	// num buffer de capacidade fixa, alocado uma vez: nunca realoca, e zerá-lo apaga a única cópia.
	bytes   []byte
	membros []membro
}

type membro struct {
	nome  string
	valor valor
}

var errJSON = errors.New("json fora do protocolo")

const profundidadeMaxima = 2

type leitor struct {
	b []byte
	i int
	// textos guarda todo buffer de texto decodificado, para `zerarTudo` apagar o que sobrar.
	textos [][]byte
}

// lerObjeto lê o documento inteiro: um objeto, e nada depois dele além de espaço.
func lerDocumento(b []byte) (valor, *leitor, error) {
	l := &leitor{b: b}
	if !utf8.Valid(b) {
		return valor{}, l, errJSON
	}
	l.espaco()
	v, err := l.objeto(1)
	if err != nil {
		return valor{}, l, err
	}
	l.espaco()
	if l.i != len(l.b) {
		return valor{}, l, errJSON
	}
	return v, l, nil
}

func (l *leitor) zerarTudo() {
	for _, t := range l.textos {
		Zerar(t[:cap(t)])
	}
	l.textos = nil
}

// zerarMenos apaga todos os textos lidos, menos o que foi entregue a quem chamou (o PIN).
func (l *leitor) zerarMenos(guardado []byte) {
	for _, t := range l.textos {
		if guardado != nil && cap(t) > 0 && cap(guardado) > 0 && &t[:cap(t)][0] == &guardado[:cap(guardado)][0] {
			continue
		}
		Zerar(t[:cap(t)])
	}
	l.textos = nil
}

func (l *leitor) espaco() {
	for l.i < len(l.b) {
		switch l.b[l.i] {
		case ' ', '\t', '\n', '\r':
			l.i++
		default:
			return
		}
	}
}

func (l *leitor) espera(c byte) error {
	if l.i >= len(l.b) || l.b[l.i] != c {
		return errJSON
	}
	l.i++
	return nil
}

func (l *leitor) objeto(profundidade int) (valor, error) {
	if err := l.espera('{'); err != nil {
		return valor{}, err
	}
	v := valor{tipo: tipoObjeto}
	l.espaco()
	if l.i < len(l.b) && l.b[l.i] == '}' {
		l.i++
		return v, nil
	}
	for {
		l.espaco()
		if l.i >= len(l.b) || l.b[l.i] != '"' {
			return valor{}, errJSON
		}
		nome, err := l.texto()
		if err != nil {
			return valor{}, err
		}
		for _, m := range v.membros {
			if m.nome == string(nome) {
				return valor{}, errJSON
			}
		}
		l.espaco()
		if err := l.espera(':'); err != nil {
			return valor{}, err
		}
		l.espaco()
		mv, err := l.valor(profundidade)
		if err != nil {
			return valor{}, err
		}
		v.membros = append(v.membros, membro{nome: string(nome), valor: mv})
		l.espaco()
		if l.i >= len(l.b) {
			return valor{}, errJSON
		}
		switch l.b[l.i] {
		case ',':
			l.i++
		case '}':
			l.i++
			return v, nil
		default:
			return valor{}, errJSON
		}
	}
}

func (l *leitor) valor(profundidade int) (valor, error) {
	if l.i >= len(l.b) {
		return valor{}, errJSON
	}
	switch c := l.b[l.i]; {
	case c == '"':
		t, err := l.texto()
		if err != nil {
			return valor{}, err
		}
		return valor{tipo: tipoTexto, bytes: t}, nil
	case c == '{':
		if profundidade >= profundidadeMaxima {
			return valor{}, errJSON
		}
		return l.objeto(profundidade + 1)
	case c == '-' || (c >= '0' && c <= '9'):
		return l.numero()
	default:
		return valor{}, errJSON
	}
}

// numero lê o literal pela gramática do JSON; quem usa decide o que aceita (o `v` só aceita `1`).
func (l *leitor) numero() (valor, error) {
	inicio := l.i
	digitos := func() int {
		n := 0
		for l.i < len(l.b) && l.b[l.i] >= '0' && l.b[l.i] <= '9' {
			l.i++
			n++
		}
		return n
	}
	if l.b[l.i] == '-' {
		l.i++
	}
	if l.i < len(l.b) && l.b[l.i] == '0' {
		l.i++
	} else if digitos() == 0 {
		return valor{}, errJSON
	}
	if l.i < len(l.b) && l.b[l.i] == '.' {
		l.i++
		if digitos() == 0 {
			return valor{}, errJSON
		}
	}
	if l.i < len(l.b) && (l.b[l.i] == 'e' || l.b[l.i] == 'E') {
		l.i++
		if l.i < len(l.b) && (l.b[l.i] == '+' || l.b[l.i] == '-') {
			l.i++
		}
		if digitos() == 0 {
			return valor{}, errJSON
		}
	}
	return valor{tipo: tipoNumero, bytes: l.b[inicio:l.i]}, nil
}

func hex4(b []byte) (rune, bool) {
	if len(b) < 4 {
		return 0, false
	}
	var r rune
	for _, c := range b[:4] {
		r <<= 4
		switch {
		case c >= '0' && c <= '9':
			r |= rune(c - '0')
		case c >= 'a' && c <= 'f':
			r |= rune(c-'a') + 10
		case c >= 'A' && c <= 'F':
			r |= rune(c-'A') + 10
		default:
			return 0, false
		}
	}
	return r, true
}

// texto lê uma string JSON (o cursor está no `"` de abertura) e a decodifica num buffer novo de
// capacidade igual ao tamanho bruto: o decodificado nunca é maior que o bruto, então o buffer não
// realoca e não deixa cópia parcial para trás. Controle cru, escape desconhecido e surrogate solto
// são recusados.
func (l *leitor) texto() ([]byte, error) {
	l.i++ // abre aspas
	inicio := l.i
	fim := -1
	for j := inicio; j < len(l.b); j++ {
		if l.b[j] == '\\' {
			j++
			continue
		}
		if l.b[j] == '"' {
			fim = j
			break
		}
	}
	if fim < 0 {
		return nil, errJSON
	}
	saida := make([]byte, 0, fim-inicio)
	l.textos = append(l.textos, saida)
	bruto := l.b[inicio:fim]
	for k := 0; k < len(bruto); {
		c := bruto[k]
		if c < 0x20 {
			return nil, errJSON
		}
		if c != '\\' {
			saida = append(saida, c)
			k++
			continue
		}
		if k+1 >= len(bruto) {
			return nil, errJSON
		}
		switch bruto[k+1] {
		case '"', '\\', '/':
			saida = append(saida, bruto[k+1])
			k += 2
		case 'b':
			saida = append(saida, '\b')
			k += 2
		case 'f':
			saida = append(saida, '\f')
			k += 2
		case 'n':
			saida = append(saida, '\n')
			k += 2
		case 'r':
			saida = append(saida, '\r')
			k += 2
		case 't':
			saida = append(saida, '\t')
			k += 2
		case 'u':
			r, ok := hex4(bruto[k+2:])
			if !ok {
				return nil, errJSON
			}
			k += 6
			switch {
			case r >= 0xd800 && r <= 0xdbff:
				if k+1 >= len(bruto) || bruto[k] != '\\' || bruto[k+1] != 'u' {
					return nil, errJSON
				}
				baixo, ok := hex4(bruto[k+2:])
				if !ok || baixo < 0xdc00 || baixo > 0xdfff {
					return nil, errJSON
				}
				k += 6
				r = 0x10000 + (r-0xd800)<<10 + (baixo - 0xdc00)
			case r >= 0xdc00 && r <= 0xdfff:
				return nil, errJSON
			}
			saida = utf8.AppendRune(saida, r)
		default:
			return nil, errJSON
		}
	}
	l.textos[len(l.textos)-1] = saida
	l.i = fim + 1
	return saida, nil
}
