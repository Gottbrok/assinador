/**
 * O lado da PÁGINA de uma janela de decisão (`confirmar.html`, `permitir.html`): achar o fluxo na
 * URL, pedir os dados ao fundo (esperando enquanto o navegador ainda não disse qual aba abriu),
 * mandar a decisão, e a trava de segurança dos botões.
 *
 * A trava existe porque a janela abre por pedido de uma página web, e essa página pode cronometrar
 * um Enter ou um clique para o instante em que a janela ganha o foco. Por isso os botões só
 * respondem depois de `ATRASO_DE_SEGURANCA_MS` com a janela visível, o mesmo cuidado que os
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

/**
 * Destrava os botões depois do atraso de segurança contado a partir de a janela estar VISÍVEL (uma
 * janela que abre atrás de outra não conta o atraso enquanto a pessoa não a vê).
 */
export function destravarDepoisDoAtraso(doc: Document, botoes: readonly HTMLButtonElement[]): void {
  for (const b of botoes) b.disabled = true;
  const contar = () => {
    setTimeout(() => {
      for (const b of botoes) b.disabled = false;
    }, ATRASO_DE_SEGURANCA_MS);
  };
  if (doc.visibilityState === 'visible') {
    contar();
    return;
  }
  const quandoVisivel = () => {
    if (doc.visibilityState !== 'visible') return;
    doc.removeEventListener('visibilitychange', quandoVisivel);
    contar();
  };
  doc.addEventListener('visibilitychange', quandoVisivel);
}
