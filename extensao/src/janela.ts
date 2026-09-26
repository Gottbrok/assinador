/**
 * O lado da PÁGINA de uma janela de decisão (`confirmar.html`, `permitir.html`): achar o fluxo na
 * URL, pedir os dados ao fundo (esperando enquanto o navegador ainda não disse qual aba abriu),
 * mandar a decisão, e a trava de segurança dos botões.
 *
 * A trava existe porque a janela abre por pedido de uma página web, e essa página pode cronometrar
 * um Enter ou um clique para o instante em que a janela ganha o foco, ou pôr uma janela dela por
 * cima e sumir com ela no primeiro clique de um clique duplo. Por isso os botões só respondem depois
 * de `ATRASO_DE_SEGURANCA_MS` com a janela VISÍVEL e COM FOCO; perder um dos dois trava de novo e
 * recomeça a contagem, e a tecla repetida (Enter segurado) não conta. É o mesmo cuidado que os
 * navegadores têm nos diálogos de permissão deles.
 */

import type { DadosDaJanela } from './janelas';

export const ATRASO_DE_SEGURANCA_MS = 600;

/** Quantas vezes, e de quanto em quanto, a janela pergunta de novo quando o fundo diz `aguarde`. */
export const TENTATIVAS_DE_DADOS = 30;
export const INTERVALO_DE_DADOS_MS = 100;

const FORMA_DO_FLUXO = /^[A-Za-z0-9-]{1,64}$/;

/** O `fluxo` da URL da janela, ou `null`. */
export function lerFluxo(busca: string): string | null {
  const fluxo = new URLSearchParams(busca).get('fluxo');
  return fluxo !== null && FORMA_DO_FLUXO.test(fluxo) ? fluxo : null;
}

export type Enviar = (mensagem: unknown) => Promise<unknown>;

function ehDadosDaJanela(valor: unknown): valor is DadosDaJanela {
  if (typeof valor !== 'object' || valor === null) return false;
  const estado = (valor as { estado?: unknown }).estado;
  return estado === 'pronto' || estado === 'aguarde' || estado === 'ausente';
}

/**
 * Os dados do fluxo. Repete enquanto o fundo responde `aguarde`; resposta sem forma, fundo que não
 * responde ou tentativas esgotadas são `null` (a janela diz que expirou).
 */
export async function pedirDados(enviar: Enviar, fluxo: string, dormir: (ms: number) => Promise<void>): Promise<unknown> {
  for (let i = 0; i < TENTATIVAS_DE_DADOS; i += 1) {
    let r: unknown;
    try {
      r = await enviar({ tipo: 'dados-da-janela', fluxo });
    } catch {
      return null;
    }
    if (!ehDadosDaJanela(r) || r.estado === 'ausente') return null;
    if (r.estado === 'pronto') return r.dados;
    await dormir(INTERVALO_DE_DADOS_MS);
  }
  return null;
}

/** Manda a decisão. `false` quando o fundo não a aceitou (fluxo encerrado pelo prazo, por exemplo). */
export async function enviarDecisao(enviar: Enviar, fluxo: string, valor: unknown): Promise<boolean> {
  try {
    return (await enviar({ tipo: 'decisao', fluxo, valor })) === true;
  } catch {
    return false;
  }
}

export const dormirDeVerdade = (ms: number): Promise<void> => new Promise((r) => setTimeout(r, ms));

/** O que a trava precisa saber da janela; a de verdade é `ambienteDoDocumento`, os testes passam uma falsa. */
export interface AmbienteDaTrava {
  visivelEComFoco(): boolean;
  /** Chama o ouvinte a cada ganho ou perda de foco e de visibilidade. */
  aoMudar(ouvinte: () => void): void;
  agendar(ms: number, f: () => void): () => void;
}

export interface Trava {
  /** Para de vigiar (a decisão foi tomada): quem chamou cuida dos botões dali em diante. */
  encerrar(): void;
}

/**
 * Trava os botões (`aplicar(true)`) e destrava `ATRASO_DE_SEGURANCA_MS` depois de a janela estar
 * visível e com foco. Perder um dos dois trava de novo e recomeça a contagem. `aoPrimeiroDestrave`
 * roda uma vez, quando os botões respondem pela primeira vez: é ali que o foco vai para o controle
 * seguro (focar botão desabilitado não faz nada).
 */
export function travarAteVer(amb: AmbienteDaTrava, aplicar: (travado: boolean) => void, aoPrimeiroDestrave?: () => void): Trava {
  let travado = true;
  let encerrada = false;
  let jaDestravou = false;
  let cancelarContagem: (() => void) | null = null;

  const travar = () => {
    cancelarContagem?.();
    cancelarContagem = null;
    if (travado) return;
    travado = true;
    aplicar(true);
  };

  const avaliar = () => {
    if (encerrada) return;
    if (!amb.visivelEComFoco()) {
      travar();
      return;
    }
    if (!travado || cancelarContagem) return;
    cancelarContagem = amb.agendar(ATRASO_DE_SEGURANCA_MS, () => {
      cancelarContagem = null;
      if (encerrada || !amb.visivelEComFoco()) return;
      travado = false;
      aplicar(false);
      if (!jaDestravou) {
        jaDestravou = true;
        aoPrimeiroDestrave?.();
      }
    });
  };

  aplicar(true);
  amb.aoMudar(avaliar);
  avaliar();
  return {
    encerrar() {
      encerrada = true;
      cancelarContagem?.();
      cancelarContagem = null;
    },
  };
}

/** A janela de verdade: visível pelo `visibilityState`, com foco pelo `hasFocus()`. */
export function ambienteDoDocumento(doc: Document, win: Window): AmbienteDaTrava {
  return {
    visivelEComFoco: () => doc.visibilityState === 'visible' && doc.hasFocus(),
    aoMudar(ouvinte) {
      win.addEventListener('focus', ouvinte);
      win.addEventListener('blur', ouvinte);
      doc.addEventListener('visibilitychange', ouvinte);
    },
    agendar(ms, f) {
      const t = setTimeout(f, ms);
      return () => clearTimeout(t);
    },
  };
}

/** Tecla repetida (Enter ou espaço segurados) não aciona nada: só o toque que a pessoa deu agora. */
export function ignorarTeclaRepetida(doc: Document): void {
  doc.addEventListener(
    'keydown',
    (evento) => {
      if (evento.repeat && (evento.key === 'Enter' || evento.key === ' ')) {
        evento.preventDefault();
        evento.stopPropagation();
      }
    },
    true,
  );
}
