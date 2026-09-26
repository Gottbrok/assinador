//go:build !windows

package pkcs11

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sync"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/mensagens"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// Prazos de um filho. `listar` cabe no prazo da página (30 s) com folga; `assinar` cobre a pessoa
// digitar o PIN no teclado da leitora (caminho protegido), depois da janela de confirmação.
const (
	PrazoPadraoDeListar  = 20 * time.Second
	PrazoPadraoDeAssinar = 90 * time.Second
	// esperaDoFim é quanto o pai espera o filho sair depois de ter a resposta (C_Finalize de
	// fabricante pode travar); depois disso, mata.
	esperaDoFim = 2 * time.Second
)

// Provedor é o provedor PKCS#11: um filho por módulo, por operação.
type Provedor struct {
	Modulos        []Modulo
	Ausentes       []Modulo
	Executavel     string
	PrazoDeListar  time.Duration
	PrazoDeAssinar time.Duration
}

// NovoProvedor descobre os módulos e monta o provedor. `executavel` é o próprio programa, que se
// relança no modo `modulo`.
func NovoProvedor(executavel string, o OpcoesDeDescoberta) *Provedor {
	achados, ausentes := Descobrir(o)
	return &Provedor{Modulos: achados, Ausentes: ausentes, Executavel: executavel, PrazoDeListar: PrazoPadraoDeListar, PrazoDeAssinar: PrazoPadraoDeAssinar}
}

var _ assinatura.Provedor = (*Provedor)(nil)

var errPrazo = errors.New("prazo esgotado")

// conversar lança o filho do módulo, manda o pedido (e o PIN, num segundo quadro) e lê a resposta.
// O PIN que o pai recebeu é zerado aqui, depois de escrito no canal.
func (p *Provedor) conversar(ctx context.Context, m Modulo, pedido pedidoAoModulo, pin []byte, prazo time.Duration) (respostaDoModulo, error) {
	defer mensagens.Zerar(pin)
	ctx, cancelar := context.WithTimeout(ctx, prazo)
	defer cancelar()

	lePedido, escrevePedido, err := os.Pipe()
	if err != nil {
		return respostaDoModulo{}, err
	}
	leResposta, escreveResposta, err := os.Pipe()
	if err != nil {
		lePedido.Close()
		escrevePedido.Close()
		return respostaDoModulo{}, err
	}
	defer escrevePedido.Close()
	defer leResposta.Close()
	// A leitura tem prazo próprio, e não só o do processo: um auxiliar que a biblioteca do
	// fabricante lance herda o descritor da resposta, e o canal não fecharia com a morte do filho.
	if prazoFinal, ok := ctx.Deadline(); ok {
		_ = leResposta.SetReadDeadline(prazoFinal)
	}
	parar := context.AfterFunc(ctx, func() { _ = leResposta.SetReadDeadline(time.Now()) })
	defer parar()

	cmd := exec.CommandContext(ctx, p.Executavel, "modulo", "--caminho", m.Caminho)
	// Entrada, saída e erro padrão em /dev/null: o filho fala só pelos descritores 3 e 4.
	cmd.ExtraFiles = []*os.File{lePedido, escreveResposta}
	cmd.SysProcAttr = atributosDoFilho()
	cmd.WaitDelay = esperaDoFim
	err = cmd.Start()
	lePedido.Close()
	escreveResposta.Close()
	if err != nil {
		return respostaDoModulo{}, err
	}
	terminou := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		// O filho saiu: o que ele escreveu já está no canal. Um auxiliar da biblioteca que herdou o
		// descritor da resposta o mantém aberto, e a leitura esperaria o prazo inteiro; ela espera só
		// `esperaDoFim` pelo que ficou no canal.
		limite := time.Now().Add(esperaDoFim)
		if prazoFinal, ok := ctx.Deadline(); ok && prazoFinal.Before(limite) {
			limite = prazoFinal
		}
		_ = leResposta.SetReadDeadline(limite)
		terminou <- err
	}()

	corpo, _ := json.Marshal(pedido)
	escrita := mensagens.EscreverQuadro(escrevePedido, corpo, tetoDoPedido)
	if escrita == nil && pedido.ComPin {
		escrita = mensagens.EscreverQuadro(escrevePedido, pin, protocolo.TetoDoPin)
	}
	mensagens.Zerar(pin)
	escrevePedido.Close()

	bruto, errLeitura := mensagens.LerQuadro(leResposta, tetoDaResposta)
	if errLeitura == nil {
		select {
		case <-terminou:
		case <-time.After(esperaDoFim):
			_ = cmd.Process.Kill()
			<-terminou
		}
	} else {
		if ctx.Err() != nil {
			_ = cmd.Process.Kill()
		}
		errFim := <-terminou
		if ctx.Err() != nil {
			return respostaDoModulo{}, errPrazo
		}
		if errFim != nil {
			return respostaDoModulo{}, fmt.Errorf("o filho do módulo terminou: %v", errFim)
		}
		return respostaDoModulo{}, fmt.Errorf("o filho do módulo não respondeu: %v", errLeitura)
	}
	if escrita != nil {
		return respostaDoModulo{}, fmt.Errorf("pedido ao filho: %v", escrita)
	}
	var r respostaDoModulo
	dec := json.NewDecoder(bytes.NewReader(bruto))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return respostaDoModulo{}, fmt.Errorf("resposta do filho fora da forma: %v", err)
	}
	return r, nil
}

// erroDaConversa traduz a falha do filho (não a do módulo, que vem na resposta).
func erroDaConversa(m Modulo, err error, noPrazo protocolo.Codigo) *protocolo.Erro {
	if errors.Is(err, errPrazo) {
		return protocolo.Novo(noPrazo, "o módulo "+m.Nome+" não respondeu no prazo")
	}
	return protocolo.Novo(protocolo.ModuloFalhou, "módulo "+m.Nome+": "+err.Error())
}

// erroDaResposta traduz a recusa que o filho mandou, conferindo o código e as tentativas contra o
// vocabulário do protocolo: o que sai daqui chega à página.
func erroDaResposta(r respostaDoModulo) *protocolo.Erro {
	if r.Erro == nil {
		return protocolo.Novo(protocolo.Interno, "resposta do módulo sem ok nem erro")
	}
	if !slices.Contains(protocolo.Codigos, r.Erro.Codigo) {
		return protocolo.Novo(protocolo.Interno, "código desconhecido na resposta do módulo")
	}
	tentativas := r.Erro.Tentativas
	if tentativas != protocolo.TentativasPoucas && tentativas != protocolo.TentativasUltima {
		tentativas = ""
	}
	return &protocolo.Erro{Codigo: r.Erro.Codigo, Detalhe: r.Erro.Detalhe, Tentativas: tentativas}
}

type resultadoDeListar struct {
	modulo   Modulo
	resposta respostaDoModulo
	erro     *protocolo.Erro
}

func (p *Provedor) listarTodos(ctx context.Context) []resultadoDeListar {
	resultados := make([]resultadoDeListar, len(p.Modulos))
	var wg sync.WaitGroup
	for i, m := range p.Modulos {
		wg.Go(func() {
			r, err := p.conversar(ctx, m, pedidoAoModulo{Op: opListar}, nil, p.PrazoDeListar)
			resultados[i] = resultadoDeListar{modulo: m, resposta: r}
			switch {
			case err != nil:
				resultados[i].erro = erroDaConversa(m, err, protocolo.ModuloFalhou)
			case !r.OK:
				resultados[i].erro = erroDaResposta(r)
			}
		})
	}
	wg.Wait()
	return resultados
}

// Listar pergunta a todos os módulos ao mesmo tempo e funde os certificados repetidos por `ref`
// (SafeSign e OpenSC vendo o mesmo cartão), ficando com o do fabricante. Módulo que falhou vira
// aviso, e os outros seguem.
func (p *Provedor) Listar(ctx context.Context) ([]assinatura.Certificado, []string) {
	var certs []assinatura.Certificado
	var avisos []string
	porRef := map[string]int{}
	for _, r := range p.listarTodos(ctx) {
		if r.erro != nil {
			avisos = append(avisos, fmt.Sprintf("O programa do cartão %s falhou e foi ignorado.", r.modulo.Rotulo))
			continue
		}
		for _, c := range r.resposta.Certificados {
			if !refConfere(c) {
				continue
			}
			novo := assinatura.Certificado{
				Ref:              c.Ref,
				DER:              c.DER,
				Provedor:         "pkcs11:" + r.modulo.Nome,
				RotuloDoProvedor: r.modulo.Rotulo,
				Leitor:           c.Leitor,
				ExigePin:         c.ExigePin,
				EstadoDoPin:      c.EstadoDoPin,
				ChaveConfirmada:  c.ChaveConfirmada,
				Interno:          r.modulo,
			}
			if i, ok := porRef[c.Ref]; ok {
				if anterior, _ := certs[i].Interno.(Modulo); anterior.Generico && !r.modulo.Generico {
					certs[i] = novo
				}
				continue
			}
			porRef[c.Ref] = len(certs)
			certs = append(certs, novo)
		}
	}
	return certs, avisos
}

// Assinar pede a assinatura ao filho do módulo que listou o certificado. O PIN é zerado aqui.
func (p *Provedor) Assinar(ctx context.Context, c assinatura.Certificado, digest [32]byte, pin []byte) ([]byte, error) {
	defer mensagens.Zerar(pin)
	m, ok := c.Interno.(Modulo)
	if !ok {
		return nil, protocolo.Novo(protocolo.Interno, "certificado sem módulo")
	}
	pedido := pedidoAoModulo{Op: opAssinar, Ref: c.Ref, Digest: hex.EncodeToString(digest[:]), ComPin: len(pin) > 0}
	r, err := p.conversar(ctx, m, pedido, pin, p.PrazoDeAssinar)
	if err != nil {
		return nil, erroDaConversa(m, err, protocolo.TempoEsgotado)
	}
	if !r.OK {
		return nil, erroDaResposta(r)
	}
	if len(r.Assinatura) == 0 {
		return nil, protocolo.Novo(protocolo.ModuloFalhou, "o módulo "+m.Nome+" devolveu assinatura vazia")
	}
	return r.Assinatura, nil
}

// Diagnosticar diz, por módulo, se carregou e quantos certificados viu, e quais pedidos (do
// catálogo, do p11-kit, da configuração) não estão instalados. Os certificados vistos vão em
// `Vistos`, que não sai no JSON: quem monta o relatório os resume sem CPF.
func (p *Provedor) Diagnosticar(ctx context.Context) []assinatura.RelatorioDoProvedor {
	var saida []assinatura.RelatorioDoProvedor
	for _, r := range p.listarTodos(ctx) {
		rel := assinatura.RelatorioDoProvedor{Nome: r.modulo.Rotulo, Caminho: r.modulo.Caminho, Origem: r.modulo.Origem}
		if r.erro != nil {
			rel.Estado = assinatura.EstadoFalhou
			rel.Detalhe = r.erro.Error()
		} else {
			rel.Estado = assinatura.EstadoCarregado
			rel.Certificados = len(r.resposta.Certificados)
			if i := r.resposta.Info; i != nil {
				rel.Fabricante = i.Fabricante
				rel.Detalhe = fmt.Sprintf("%s, Cryptoki %s, biblioteca %s", i.Fabricante, i.Cryptoki, i.Biblioteca)
			}
			for _, c := range r.resposta.Certificados {
				if refConfere(c) {
					rel.Vistos = append(rel.Vistos, assinatura.Certificado{Ref: c.Ref, DER: c.DER, Provedor: "pkcs11:" + r.modulo.Nome, RotuloDoProvedor: r.modulo.Rotulo, Leitor: c.Leitor})
				}
			}
		}
		saida = append(saida, rel)
	}
	for _, m := range p.Ausentes {
		saida = append(saida, assinatura.RelatorioDoProvedor{Nome: m.Rotulo, Caminho: m.Caminho, Origem: m.Origem, Estado: assinatura.EstadoAusente})
	}
	return saida
}
