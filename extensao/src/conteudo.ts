/**
 * Script de conteúdo: a ponte entre a página e o fundo da extensão (§3.6 do plano).
 *
 * Roda no mundo ISOLADO, só no quadro de topo, desde o início do documento. Aceita só a mensagem que
 * a própria janela postou (`event.source === window`), com a origem da própria página, o canal e a
 * versão do protocolo certos, um `id` com forma e até 64 KiB; repassa ao fundo com a origem da
 * página, e devolve a resposta com `targetOrigin` igual à origem da página. Todo o resto passa reto,
 * inclusive as respostas que este mesmo script posta (canal de resposta).
 *
 * Cada pedido vai por uma PORTA própria (`runtime.connect`), e não por `sendMessage`: quando a página
 * fecha ou navega, este script morre e a porta cai, e é assim que o fundo sabe que ninguém mais
 * espera aquela resposta (ele fecha a janela de confirmação e solta a assinatura). O prazo da página
 * é cobrado AQUI, no mesmo relógio dela (`PRAZOS_NA_PONTE_MS`): vencido, a página recebe
 * `tempo-esgotado` em vez de desistir sozinha.
 *
 * O script não decide nada: permissão, origem aceita e a conversa com o programa são do fundo, que
 * confere a origem de novo pelo que o NAVEGADOR diz do remetente.
 */

import { navegador } from './api';
import {
  CANAL_PEDIDO,
  CANAL_RESPOSTA,
  falha,
  FORMA_DO_ID,
  OPERACOES_DA_PAGINA,
  PORTA_DO_PEDIDO,
  PRAZOS_NA_PONTE_MS,
  PROTOCOLO,
  TAMANHO_MAXIMO_DA_MENSAGEM,
  type OperacaoDaPagina,
  type Resultado,
} from './protocolo';

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

/** A porta de um pedido até o fundo, no que a ponte usa. `abrir` pode lançar (extensão atualizada). */
export interface CanalDoFundo {
  enviar(mensagem: unknown): void;
  aoReceber(ouvinte: (mensagem: unknown) => void): void;
  aoDesligar(ouvinte: () => void): void;
  desligar(): void;
}

export type Agendar = (ms: number, f: () => void) => () => void;

const agendarDeVerdade: Agendar = (ms, f) => {
  const t = setTimeout(f, ms);
  return () => clearTimeout(t);
};

const CHAVES_DO_PEDIDO = new Set(['canal', 'v', 'id', 'op', 'dados']);

function ehObjeto(valor: unknown): valor is Record<string, unknown> {
  return typeof valor === 'object' && valor !== null && !Array.isArray(valor);
}

/**
 * O pedido que a extensão aceita da página, ou `null` para passar reto. A mensagem é COPIADA por JSON
 * antes de qualquer leitura: a cópia é o que se mede contra o teto, só tem dado de JSON, e é um
 * objeto deste script (no Firefox, `event.data` chega embrulhado pela fronteira com a página). A
 * operação e os dados NÃO são conferidos aqui: o fundo responde `protocolo` para o que vier sem
 * forma, e a página fica sabendo, em vez de esperar o prazo.
 */
export function lerPedido(evento: EventoDaJanela, janela: JanelaDoConteudo): Omit<PedidoParaOFundo, 'tipo' | 'origem'> | null {
  if (evento.source !== janela || evento.origin !== janela.location.origin) return null;
  let texto: string | undefined;
  try {
    texto = JSON.stringify(evento.data);
  } catch {
    return null;
  }
  if (typeof texto !== 'string' || texto.length > TAMANHO_MAXIMO_DA_MENSAGEM) return null;
  const m: unknown = JSON.parse(texto);
  if (!ehObjeto(m) || m.canal !== CANAL_PEDIDO || m.v !== PROTOCOLO) return null;
  if (typeof m.id !== 'string' || !FORMA_DO_ID.test(m.id)) return null;
  if (Object.keys(m).some((k) => !CHAVES_DO_PEDIDO.has(k))) return null;
  return m.dados === undefined ? { id: m.id, op: m.op } : { id: m.id, op: m.op, dados: m.dados };
}

function ehResultado(valor: unknown): valor is Resultado {
  if (!ehObjeto(valor)) return false;
  if (valor.ok === true) return ehObjeto(valor.dados);
  return valor.ok === false && ehObjeto(valor.erro) && typeof valor.erro.codigo === 'string';
}

function prazoDa(op: unknown): number {
  return typeof op === 'string' && (OPERACOES_DA_PAGINA as readonly string[]).includes(op) ? PRAZOS_NA_PONTE_MS[op as OperacaoDaPagina] : PRAZOS_NA_PONTE_MS.listar;
}

const CONEXAO_PERDIDA = 'a conexão com a extensão caiu (ela foi atualizada ou reiniciada): recarregue a página e tente de novo';

/**
 * Liga a ponte. Cada pedido abre um canal até o fundo e espera UMA resposta; a primeira coisa que
 * acontecer responde a página, e o resto é ignorado: a resposta do fundo, o canal que cai (`interno`,
 * mandando recarregar), o canal que nem abre (idem) ou o prazo da página (`tempo-esgotado`).
 */
export function instalarPonte(janela: JanelaDoConteudo, abrirCanal: () => CanalDoFundo, agendar: Agendar = agendarDeVerdade): void {
  const origem = janela.location.origin;
  janela.addEventListener('message', (evento) => {
    const pedido = lerPedido(evento, janela);
    if (!pedido) return;
    let respondido = false;
    let canal: CanalDoFundo | null = null;
    let cancelarPrazo: () => void = () => undefined;
    const responder = (resultado: Resultado) => {
      if (respondido) return;
      respondido = true;
      cancelarPrazo();
      janela.postMessage({ canal: CANAL_RESPOSTA, v: PROTOCOLO, id: pedido.id, ...resultado }, origem);
      try {
        canal?.desligar();
      } catch {
        // O canal já tinha caído.
      }
    };
    cancelarPrazo = agendar(prazoDa(pedido.op), () => responder(falha('tempo-esgotado', 'a extensão não respondeu no prazo da página')));
    try {
      const c = abrirCanal();
      canal = c;
      c.aoReceber((m) => responder(ehResultado(m) ? m : falha('interno', 'fundo sem resposta')));
      c.aoDesligar(() => responder(falha('interno', CONEXAO_PERDIDA)));
      c.enviar({ tipo: 'pedido-da-pagina', ...pedido, origem } satisfies PedidoParaOFundo);
    } catch {
      // `runtime.connect` lança, de forma síncrona, quando a extensão foi atualizada com a página aberta.
      responder(falha('interno', CONEXAO_PERDIDA));
    }
  });
}

/** O canal de verdade: uma porta `runtime.connect` por pedido. */
export function canalDoNavegador(api: typeof chrome): CanalDoFundo {
  const porta = api.runtime.connect({ name: PORTA_DO_PEDIDO });
  return {
    enviar: (m) => porta.postMessage(m),
    aoReceber: (f) => porta.onMessage.addListener((m: unknown) => f(m)),
    aoDesligar: (f) => porta.onDisconnect.addListener(() => f()),
    desligar: () => porta.disconnect(),
  };
}

// No navegador, liga a ponte da janela ao fundo. Nos testes não há API de extensão, e nada roda.
const api = navegador();
if (api) instalarPonte(window as unknown as JanelaDoConteudo, () => canalDoNavegador(api));
