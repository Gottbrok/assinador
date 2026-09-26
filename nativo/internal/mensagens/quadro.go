// Package mensagens lê e escreve as mensagens de native messaging (4 bytes de tamanho na ordem
// nativa, depois JSON UTF-8) e decodifica o pedido da extensão com um leitor ESTRITO próprio.
//
// Por que não o `encoding/json`: o pedido de `assinar` traz o PIN, e o decodificador padrão (o v1
// e o v2) passa os textos por buffers internos e por `string`, que ninguém consegue zerar (o v2
// ainda os devolve a um `sync.Pool`). O leitor daqui decodifica cada texto num `[]byte` de
// capacidade fixa, e o do PIN vai direto para o pedido, que o zera depois do login (regra 2 do
// CLAUDE.md). De quebra, ele é estrito onde o contrato pede: chave repetida, chave desconhecida,
// maiúscula, UTF-8 inválido, surrogate solto e tipo errado são `protocolo`.
package mensagens

import (
	"encoding/binary"
	"errors"
	"io"
	"runtime"
)

// ErrQuadroGrande é a mensagem acima do teto. Depois dela o fluxo não se ressincroniza: quem lê
// encerra.
var ErrQuadroGrande = errors.New("mensagem acima do limite")

// LerQuadro lê uma mensagem. `io.EOF` só quando a entrada fechou ENTRE mensagens; fechar no meio de
// uma é `io.ErrUnexpectedEOF`. Quem recebe o corpo o zera depois de usar.
func LerQuadro(r io.Reader, limite int) ([]byte, error) {
	var cabecalho [4]byte
	if _, err := io.ReadFull(r, cabecalho[:]); err != nil {
		return nil, err
	}
	n := binary.NativeEndian.Uint32(cabecalho[:])
	if uint64(n) > uint64(limite) {
		return nil, ErrQuadroGrande
	}
	corpo := make([]byte, n)
	if _, err := io.ReadFull(r, corpo); err != nil {
		Zerar(corpo)
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return corpo, nil
}

// EscreverQuadro escreve uma mensagem numa única chamada de `Write` (cabeçalho e corpo juntos),
// para duas respostas nunca se intercalarem mesmo que o chamador esqueça a trava.
func EscreverQuadro(w io.Writer, corpo []byte, limite int) error {
	if len(corpo) > limite {
		return ErrQuadroGrande
	}
	buf := make([]byte, 4+len(corpo))
	binary.NativeEndian.PutUint32(buf, uint32(len(corpo)))
	copy(buf[4:], corpo)
	_, err := w.Write(buf)
	Zerar(buf)
	return err
}

// Zerar apaga um buffer que pode ter tido PIN. O `KeepAlive` impede que o compilador trate a
// escrita como morta.
func Zerar(b []byte) {
	clear(b)
	runtime.KeepAlive(b)
}
