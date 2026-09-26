/**
 * As janelas de decisão da extensão: `permitir.html` (a pessoa autoriza um endereço) e
 * `confirmar.html` (a pessoa confere o documento e assina).
 *
 * Cada janela é uma página da PRÓPRIA extensão, aberta num popup; a página que pediu a operação
 * não a alcança (outra origem, outra janela). A decisão chega ao fundo por mensagem, e só vale a
 * que vier da aba que o fundo abriu para aquele fluxo, pela origem da extensão: um script de
 * conteúdo, ou outra aba, não decide por ela. Janela fechada é uma resposta (`fechada`); o prazo,
 * outra (`prazo`), e aí o fundo fecha a janela.
 */

import type { Relogio } from './nativo';

/**
 * Como a espera terminou: a decisão da janela, a janela fechada pela pessoa, o prazo, ou a janela
 * que nem abriu (`falhou`, que não é escolha da pessoa e por isso não vira `cancelado`).
 */
export type Desfecho<T> = { tipo: 'decisao'; valor: T } | { tipo: 'fechada' } | { tipo: 'prazo' } | { tipo: 'falhou' };

export type DadosDaJanela = { estado: 'pronto'; dados: unknown } | { estado: 'aguarde' } | { estado: 'ausente' };

export interface ControleDeJanelas {
  /** Abre a página (url completa) num popup; devolve a janela e a aba. */
  abrir(url: string, largura: number, altura: number): Promise<{ janelaId: number; abaId: number }>;
  fechar(janelaId: number): Promise<void>;
  aoFechar(ouvinte: (janelaId: number) => void): void;
}

interface Espera {
  janelaId: number;
  abaId: number;
  dados: unknown;
  resolver: (d: Desfecho<unknown>) => void;
  cancelarPrazo: () => void;
}

export interface Janelas {
  /**
   * Abre `pagina?fluxo=<id>` e espera. `dados` é o que a página pede ao abrir (`dadosDaJanela`).
   * Nunca rejeita: janela que não abre é `falhou`.
   */
  esperar<T>(pagina: string, largura: number, altura: number, dados: unknown, prazoMs: number): Promise<Desfecho<T>>;
  /**
   * Os dados do fluxo, para a janela que os pede. `aguarde` enquanto o navegador ainda não disse
   * qual aba abriu (a página da janela pode carregar antes de `windows.create` devolver), e a janela
   * pergunta de novo; `ausente` para fluxo que não existe ou aba que não é a do fluxo.
   */
  dadosDaJanela(fluxo: string, abaId: number | undefined): DadosDaJanela;
  /** A decisão da janela; `false` quando não é a aba do fluxo (e nada acontece). */
  decidir(fluxo: string, abaId: number | undefined, valor: unknown): boolean;
}

export function criarJanelas(controle: ControleDeJanelas, urlDaPagina: (pagina: string) => string, novoId: () => string, relogio: Relogio): Janelas {
  const esperas = new Map<string, Espera>();
  /** Fluxos cuja janela o navegador ainda está abrindo (a aba ainda não é conhecida). */
  const abrindo = new Set<string>();
  /** Janelas fechadas enquanto algum fluxo abria: a que fechou antes de o fundo saber dela conta como fechada. */
  const fechadasEnquantoAbria = new Set<number>();

  controle.aoFechar((janelaId) => {
    if (abrindo.size > 0) fechadasEnquantoAbria.add(janelaId);
    for (const [fluxo, e] of esperas) {
      if (e.janelaId !== janelaId) continue;
      esperas.delete(fluxo);
      e.cancelarPrazo();
      e.resolver({ tipo: 'fechada' });
    }
  });

  return {
    esperar<T>(pagina: string, largura: number, altura: number, dados: unknown, prazoMs: number) {
      return new Promise<Desfecho<T>>((resolver) => {
        const fluxo = novoId();
        const url = `${urlDaPagina(pagina)}?fluxo=${encodeURIComponent(fluxo)}`;
        abrindo.add(fluxo);
        const aberta = () => {
          abrindo.delete(fluxo);
          if (abrindo.size === 0) fechadasEnquantoAbria.clear();
        };
        controle.abrir(url, largura, altura).then(
          ({ janelaId, abaId }) => {
            const fechouAntes = fechadasEnquantoAbria.has(janelaId);
            aberta();
            if (fechouAntes) {
              resolver({ tipo: 'fechada' });
              return;
            }
            const cancelarPrazo = relogio.agendar(prazoMs, () => {
              if (!esperas.delete(fluxo)) return;
              resolver({ tipo: 'prazo' });
              void controle.fechar(janelaId).catch(() => undefined);
            });
            esperas.set(fluxo, { janelaId, abaId, dados, resolver: resolver as (d: Desfecho<unknown>) => void, cancelarPrazo });
          },
          () => {
            aberta();
            resolver({ tipo: 'falhou' });
          },
        );
      });
    },

    dadosDaJanela(fluxo, abaId) {
      if (abrindo.has(fluxo)) return { estado: 'aguarde' };
      const e = esperas.get(fluxo);
      return e && abaId !== undefined && e.abaId === abaId ? { estado: 'pronto', dados: e.dados } : { estado: 'ausente' };
    },

    decidir(fluxo, abaId, valor) {
      const e = esperas.get(fluxo);
      if (!e || abaId === undefined || e.abaId !== abaId) return false;
      esperas.delete(fluxo);
      e.cancelarPrazo();
      e.resolver({ tipo: 'decisao', valor });
      void controle.fechar(e.janelaId).catch(() => undefined);
      return true;
    },
  };
}

/** O controle de janelas de verdade (`windows.create` num popup). */
export function controleDoNavegador(api: typeof chrome): ControleDeJanelas {
  return {
    async abrir(url, largura, altura) {
      const janela = await api.windows.create({ url, type: 'popup', width: largura, height: altura, focused: true });
      const abaId = janela?.tabs?.[0]?.id;
      if (janela?.id === undefined || abaId === undefined) throw new Error('janela sem aba');
      return { janelaId: janela.id, abaId };
    },
    async fechar(janelaId) {
      await api.windows.remove(janelaId);
    },
    aoFechar(ouvinte) {
      api.windows.onRemoved.addListener(ouvinte);
    },
  };
}
