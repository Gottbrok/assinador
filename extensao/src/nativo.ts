/**
 * A conversa da extensão com o programa nativo (native messaging, `br.com.confidata.assinador`).
 *
 * Cada FLUXO abre a sua porta (`connectNative` lança um processo do programa) e a fecha quando
 * termina: `ola`, `listar` e `diagnostico` são um pedido só; `assinar` é `conferir`, a decisão da
 * pessoa na janela e `assinar`, na MESMA porta. A porta aberta é também o que mantém vivo o service
 * worker do Chrome enquanto a pessoa lê a janela (Chrome 116 em diante), e por isso ela não fecha
 * por ociosidade no meio de um fluxo: fecha no fim dele, sempre, inclusive na falha.
 *
 * A resposta do programa é lida com a mesma exigência da página: `v` certo, o `id` do pedido, e
 * `ok` com `dados` ou `erro` com código do protocolo. Qualquer outra coisa é `protocolo`.
 */

import { ehCodigoDeErro, PRAZOS_DO_PROGRAMA_MS, PROTOCOLO, type OperacaoDoPrograma, type Resultado } from './protocolo';

/** O nome do host nos manifestos dos navegadores (`origem.NomeDoHost` no programa). */
export const NOME_DO_HOST = 'br.com.confidata.assinador';

/** A porta, no que esta camada usa. `aoDesligar` recebe a mensagem de erro do navegador, se houver. */
export interface PortaNativa {
  enviar(mensagem: unknown): void;
  aoReceber(ouvinte: (mensagem: unknown) => void): void;
  aoDesligar(ouvinte: (erro: string | undefined) => void): void;
  desligar(): void;
}

export interface Programa {
  /**
   * Um pedido; resolve com o resultado (nunca rejeita). O prazo é o menor entre o teto da operação
   * (`PRAZOS_DO_PROGRAMA_MS`) e `prazoMs`, o que resta do orçamento do fluxo.
   */
  pedir(op: OperacaoDoPrograma, origem: string, dados?: Record<string, unknown>, prazoMs?: number): Promise<Resultado>;
  /** Fecha a porta (o processo do programa sai quando a entrada fecha). */
  fechar(): void;
  /**
   * O programa saiu (ou nem abriu) SEM ninguém ter pedido: o ouvinte recebe a recusa que um pedido
   * teria recebido. Quem espera a pessoa numa janela usa isto para não descobrir só depois do clique.
   */
  aoCair(ouvinte: (recusa: Resultado) => void): void;
}

function ehObjeto(valor: unknown): valor is Record<string, unknown> {
  return typeof valor === 'object' && valor !== null && !Array.isArray(valor);
}

/**
 * O programa não está instalado para este navegador (ou o manifesto do host não existe). As
 * mensagens são as dos navegadores: "Specified native messaging host not found." no Chrome e no
 * Edge, "No such native application ..." no Firefox.
 */
export function programaAusente(erro: string | undefined): boolean {
  return typeof erro === 'string' && /not found|no such native application/i.test(erro);
}

/** A resposta do programa, conferida na forma. */
export function lerResposta(m: unknown, id: string): Resultado {
  if (!ehObjeto(m) || m.v !== PROTOCOLO || m.id !== id) return { ok: false, erro: { codigo: 'protocolo', detalhe: 'resposta do programa sem forma' } };
  if (m.ok === true && ehObjeto(m.dados)) return { ok: true, dados: m.dados };
  if (m.ok === false && ehObjeto(m.erro) && ehCodigoDeErro(m.erro.codigo)) {
    return m.erro.detalhe === undefined ? { ok: false, erro: { codigo: m.erro.codigo } } : { ok: false, erro: { codigo: m.erro.codigo, detalhe: m.erro.detalhe } };
  }
  return { ok: false, erro: { codigo: 'protocolo', detalhe: 'resposta do programa sem forma' } };
}

export interface Relogio {
  /** Milissegundos de um relógio que só anda para a frente. */
  agora(): number;
  agendar(ms: number, f: () => void): () => void;
}

export const relogioReal: Relogio = {
  agora: () => performance.now(),
  agendar(ms, f) {
    const t = setTimeout(f, ms);
    return () => clearTimeout(t);
  },
};

/**
 * Abre uma porta e devolve o programa sobre ela. Um pedido por vez (o programa responde `ocupado`
 * ao segundo, e o fluxo nunca manda dois). Porta que cai com pedido pendente resolve o pedido com
 * `nativo-ausente` (programa não instalado) ou `modulo-falhou` (o processo morreu).
 */
export function abrirPrograma(conectar: () => PortaNativa, novoId: () => string, relogio: Relogio = relogioReal): Programa {
  let porta: PortaNativa | null = null;
  let caiu: string | null = null;
  let pendente: { id: string; resolver: (r: Resultado) => void; cancelarPrazo: () => void } | null = null;
  const ouvintesDaQueda: ((recusa: Resultado) => void)[] = [];

  const terminar = (r: Resultado) => {
    if (!pendente) return;
    const p = pendente;
    pendente = null;
    p.cancelarPrazo();
    p.resolver(r);
  };

  /** Fecha a porta corrente (o processo do programa sai quando a entrada fecha) e esquece dela. */
  const descartarPorta = () => {
    if (!porta) return;
    const p = porta;
    porta = null;
    try {
      p.desligar();
    } catch {
      // A porta já tinha caído.
    }
  };

  const garantirPorta = (): PortaNativa => {
    if (porta) return porta;
    const p = conectar();
    // Os ouvintes só valem enquanto esta é a porta corrente: a porta descartada não fala mais.
    p.aoReceber((m) => {
      if (porta !== p || !pendente) return;
      // Resposta a pedido que não é o corrente: o fluxo não se ressincroniza.
      terminar(lerResposta(m, pendente.id));
    });
    p.aoDesligar((erro) => {
      if (porta !== p) return;
      caiu = erro ?? '';
      porta = null;
      const recusa = recusaDaQueda(caiu);
      terminar(recusa);
      for (const ouvinte of ouvintesDaQueda.splice(0)) ouvinte(recusa);
    });
    porta = p;
    return p;
  };

  return {
    pedir(op, origem, dados, prazoMs) {
      return new Promise<Resultado>((resolver) => {
        const prazo = Math.min(PRAZOS_DO_PROGRAMA_MS[op], prazoMs ?? Number.POSITIVE_INFINITY);
        if (prazo <= 0) {
          resolver({ ok: false, erro: { codigo: 'tempo-esgotado', detalhe: `sem tempo para ${op}` } });
          return;
        }
        if (pendente) {
          resolver({ ok: false, erro: { codigo: 'ocupado', detalhe: 'outro pedido em curso nesta porta' } });
          return;
        }
        if (caiu !== null) {
          resolver(recusaDaQueda(caiu));
          return;
        }
        const id = novoId();
        // Prazo vencido descarta a porta: o programa pode estar no meio do pedido, e a resposta
        // atrasada não pode chegar ao pedido seguinte. O próximo pedido abre outra porta.
        const cancelarPrazo = relogio.agendar(prazo, () => {
          terminar({ ok: false, erro: { codigo: 'tempo-esgotado', detalhe: `o programa não respondeu ${op}` } });
          descartarPorta();
        });
        pendente = { id, resolver, cancelarPrazo };
        try {
          garantirPorta().enviar({ v: PROTOCOLO, id, op, origem, ...(dados ? { dados } : {}) });
        } catch (erro) {
          terminar({ ok: false, erro: { codigo: 'nativo-ausente', detalhe: erro instanceof Error ? erro.message : String(erro) } });
        }
      });
    },
    fechar() {
      // Quem fecha não quer mais saber de queda: a porta que ele mesmo fechou não é queda.
      ouvintesDaQueda.length = 0;
      terminar({ ok: false, erro: { codigo: 'cancelado', detalhe: 'fluxo encerrado' } });
      descartarPorta();
    },
    aoCair(ouvinte) {
      if (caiu !== null) ouvinte(recusaDaQueda(caiu));
      else ouvintesDaQueda.push(ouvinte);
    },
  };
}

/** A recusa de um programa que saiu: ausente (o navegador não o achou) ou falhou (o processo morreu). */
function recusaDaQueda(erro: string): Resultado {
  return programaAusente(erro) ? { ok: false, erro: { codigo: 'nativo-ausente', detalhe: erro } } : { ok: false, erro: { codigo: 'modulo-falhou', detalhe: erro || 'o programa saiu' } };
}

/** A porta de verdade (`runtime.connectNative`), com o erro que cada navegador dá quando ela cai. */
export function portaDoNavegador(api: typeof chrome): PortaNativa {
  const porta = api.runtime.connectNative(NOME_DO_HOST);
  return {
    enviar: (m) => porta.postMessage(m),
    aoReceber: (f) => porta.onMessage.addListener((m: unknown) => f(m)),
    aoDesligar: (f) =>
      porta.onDisconnect.addListener(() => {
        // Chrome e Edge põem o motivo em `runtime.lastError`; o Firefox, em `port.error`.
        const erroDaPorta = (porta as unknown as { error?: { message?: string } }).error?.message;
        f(api.runtime.lastError?.message ?? erroDaPorta);
      }),
    desligar: () => porta.disconnect(),
  };
}
