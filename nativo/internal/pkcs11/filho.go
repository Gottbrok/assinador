//go:build !windows

package pkcs11

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/mensagens"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// O canal entre o programa (pai) e o filho de um módulo. O filho lê o pedido pelo descritor 3 e
// responde pelo 4, com o mesmo quadro de native messaging; a entrada e a saída padrão dele apontam
// para /dev/null, porque biblioteca de fabricante que imprime na tela corromperia um canal que
// fosse a saída padrão. O PIN, quando há, vai num SEGUNDO quadro, em bytes crus: nunca passa por
// JSON nem por `string`.
const (
	descritorDoPedido   = 3
	descritorDaResposta = 4
	tetoDoPedido        = 4 * 1024
	tetoDaResposta      = 2 * 1024 * 1024
)

type pedidoAoModulo struct {
	Op     string `json:"op"`
	Ref    string `json:"ref,omitempty"`
	Digest string `json:"digest,omitempty"`
	ComPin bool   `json:"comPin,omitempty"`
}

type erroDoModulo struct {
	Codigo     protocolo.Codigo `json:"codigo"`
	Detalhe    string           `json:"detalhe,omitempty"`
	Tentativas string           `json:"tentativas,omitempty"`
}

type respostaDoModulo struct {
	OK           bool                  `json:"ok"`
	Erro         *erroDoModulo         `json:"erro,omitempty"`
	Info         *infoDoModulo         `json:"info,omitempty"`
	Certificados []certificadoDoModulo `json:"certificados,omitempty"`
	Assinatura   []byte                `json:"assinatura,omitempty"`
}

const (
	opListar  = "listar"
	opAssinar = "assinar"
)

func falhaDoModulo(e *protocolo.Erro) respostaDoModulo {
	return respostaDoModulo{Erro: &erroDoModulo{Codigo: e.Codigo, Detalhe: e.Detalhe, Tentativas: e.Tentativas}}
}

// ExecutarModulo é o modo `modulo` do programa: atende UM pedido para o módulo `caminho` e sai. O
// módulo só é carregado depois de o pedido ser lido e conferido.
func ExecutarModulo(caminho string) int {
	entrada := os.NewFile(descritorDoPedido, "pedido")
	saida := os.NewFile(descritorDaResposta, "resposta")
	if entrada == nil || saida == nil {
		return 2
	}
	defer entrada.Close()
	defer saida.Close()

	corpo, err := mensagens.LerQuadro(entrada, tetoDoPedido)
	if err != nil {
		return 2
	}
	var p pedidoAoModulo
	dec := json.NewDecoder(bytes.NewReader(corpo))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil || dec.More() {
		return responder(saida, falhaDoModulo(protocolo.Novo(protocolo.Interno, "pedido ao módulo fora da forma")))
	}
	var pin []byte
	if p.ComPin {
		pin, err = mensagens.LerQuadro(entrada, protocolo.TetoDoPin)
		if err != nil {
			return 2
		}
	}
	resposta, limpar := atender(caminho, p, pin)
	mensagens.Zerar(pin)
	// A resposta sai ANTES da limpeza (logout, sessões, C_Finalize, dlclose): módulo de fabricante
	// que trava ou cai ao encerrar não custa a resposta, e o pai mata o filho depois de
	// `esperaDoFim`.
	codigo := responder(saida, resposta)
	if limpar != nil {
		limpar()
	}
	return codigo
}

func responder(saida *os.File, r respostaDoModulo) int {
	b, err := json.Marshal(r)
	if err != nil || len(b) > tetoDaResposta {
		b, _ = json.Marshal(falhaDoModulo(protocolo.Novo(protocolo.ModuloFalhou, "resposta do módulo acima do teto")))
	}
	if mensagens.EscreverQuadro(saida, b, tetoDaResposta) != nil {
		return 2
	}
	return 0
}

// atender faz o trabalho e devolve a resposta e a limpeza, que quem chama roda DEPOIS de responder.
func atender(caminho string, p pedidoAoModulo, pin []byte) (respostaDoModulo, func()) {
	if p.Op != opListar && p.Op != opAssinar {
		return falhaDoModulo(protocolo.Novo(protocolo.Interno, "operação desconhecida no módulo")), nil
	}
	var digest [32]byte
	if p.Op == opAssinar {
		d, err := hex.DecodeString(p.Digest)
		if err != nil || len(d) != len(digest) || len(p.Ref) != 64 {
			return falhaDoModulo(protocolo.Novo(protocolo.Interno, "pedido de assinatura fora da forma")), nil
		}
		copy(digest[:], d)
	}
	s, e := abrirModulo(caminho)
	if e != nil {
		return falhaDoModulo(e), nil
	}
	switch p.Op {
	case opListar:
		info := s.info()
		certs, e := s.listar()
		if e != nil {
			return falhaDoModulo(e), s.fechar
		}
		return respostaDoModulo{OK: true, Info: &info, Certificados: certs}, s.fechar
	default:
		assinada, e := s.assinar(p.Ref, digest, pin)
		if e != nil {
			return falhaDoModulo(e), s.fechar
		}
		return respostaDoModulo{OK: true, Assinatura: assinada}, s.fechar
	}
}

// refConfere diz se a ref que o filho mandou é mesmo o SHA-256 do DER que ele mandou.
func refConfere(c certificadoDoModulo) bool {
	return assinatura.Ref(c.DER) == c.Ref
}
