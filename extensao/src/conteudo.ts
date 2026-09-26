/**
 * Script de conteúdo: a ponte entre a página e o fundo da extensão (§3.6 do plano).
 *
 * Roda no mundo ISOLADO, só no quadro de topo, desde o início do documento. Aceita só a mensagem que
 * a própria janela postou (`event.source === window`), com a origem da própria página, o canal e a
 * versão do protocolo certos, um `id` com forma e até 64 KiB; repassa ao fundo com a origem da
 * página, e devolve a resposta com `targetOrigin` igual à origem da página. Todo o resto passa reto,
 * inclusive as respostas que este mesmo script posta (canal de resposta).
 *
 * O script não decide nada: permissão, origem aceita e a conversa com o programa são do fundo, que
 * confere a origem de novo pelo que o NAVEGADOR diz do remetente.
 */

import { navegador } from './api';
import { CANAL_PEDIDO, CANAL_RESPOSTA, FORMA_DO_ID, PROTOCOLO, TAMANHO_MAXIMO_DA_MENSAGEM, type Resultado } from './protocolo';

/** O que o fundo recebe de um pedido da página. */
export interface PedidoParaOFundo {
  tipo: 'pedido-da-pagina';
  id: string;
  op: unknown;
  dados?: unknown;
  origem: string;
}

export interface EventoDaJanela {
  readonly data: unknown;
  readonly source: unknown;
  readonly origin: string;
}

export interface JanelaDoConteudo {
  readonly location: { readonly origin: string };
  addEventListener(tipo: 'message', ouvinte: (evento: EventoDaJanela) => void): void;
  postMessage(mensagem: unknown, origemAlvo: string): void;
}

const CHAVES_DO_PEDIDO = new Set(['canal', 'v', 'id', 'op', 'dados']);

function ehObjeto(valor: unknown): valor is Record<string, unknown> {
  return typeof valor === 'object' && valor !== null && !Array.isArray(valor);
}

function tamanho(valor: unknown): number {
  try {
    return JSON.stringify(valor).length;
  } catch {
    return Number.POSITIVE_INFINITY;
  }
}

/**
 * O pedido que a extensão aceita da página, ou `null` para passar reto. A operação e os dados NÃO
 * são conferidos aqui: o fundo responde `protocolo` para o que vier sem forma, e a página fica
 * sabendo, em vez de esperar o prazo.
 */
export function lerPedido(evento: EventoDaJanela, janela: JanelaDoConteudo): Omit<PedidoParaOFundo, 'tipo' | 'origem'> | null {
  if (evento.source !== janela || evento.origin !== janela.location.origin) return null;
  const m = evento.data;
  if (!ehObjeto(m) || m.canal !== CANAL_PEDIDO || m.v !== PROTOCOLO) return null;
  if (typeof m.id !== 'string' || !FORMA_DO_ID.test(m.id)) return null;
  if (Object.keys(m).some((k) => !CHAVES_DO_PEDIDO.has(k))) return null;
  if (tamanho(m) > TAMANHO_MAXIMO_DA_MENSAGEM) return null;
  return m.dados === undefined ? { id: m.id, op: m.op } : { id: m.id, op: m.op, dados: m.dados };
}

function ehResultado(valor: unknown): valor is Resultado {
  if (!ehObjeto(valor)) return false;
  if (valor.ok === true) return ehObjeto(valor.dados);
  return valor.ok === false && ehObjeto(valor.erro) && typeof valor.erro.codigo === 'string';
}

/**
 * Liga a ponte. `enviarAoFundo` devolve o `Resultado` do fundo; qualquer outra coisa (o fundo sem
 * ouvinte, a extensão recarregada com a página aberta) vira `interno` com o motivo, para a página
 * não esperar o prazo à toa.
 */
export function instalarPonte(janela: JanelaDoConteudo, enviarAoFundo: (pedido: PedidoParaOFundo) => Promise<unknown>): void {
  const origem = janela.location.origin;
  janela.addEventListener('message', (evento) => {
    const pedido = lerPedido(evento, janela);
    if (!pedido) return;
    const responder = (resultado: Resultado) => {
      janela.postMessage({ canal: CANAL_RESPOSTA, v: PROTOCOLO, id: pedido.id, ...resultado }, origem);
    };
    enviarAoFundo({ tipo: 'pedido-da-pagina', ...pedido, origem }).then(
      (resposta) => responder(ehResultado(resposta) ? resposta : { ok: false, erro: { codigo: 'interno', detalhe: 'fundo sem resposta' } }),
      () => responder({ ok: false, erro: { codigo: 'interno', detalhe: 'a extensão foi atualizada: recarregue a página' } }),
    );
  });
}

// No navegador, liga a ponte da janela ao fundo. Nos testes não há API de extensão, e nada roda.
const api = navegador();
if (api) instalarPonte(window as unknown as JanelaDoConteudo, (pedido) => api.runtime.sendMessage(pedido));
