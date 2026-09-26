/**
 * Apoio dos testes: relógio manual, armazenamento em memória, porta nativa e programa falsos,
 * janelas roteirizadas. Nada aqui entra no build.
 */

import type { Desfecho, DadosDaJanela, Janelas } from '../src/janelas';
import type { PortaNativa, Relogio } from '../src/nativo';
import type { Armazenamento } from '../src/permissoes';

/** Relógio que só anda quando o teste manda. */
export class RelogioManual implements Relogio {
  private t = 0;
  private fila: { quando: number; f: () => void; vivo: boolean }[] = [];

  agora(): number {
    return this.t;
  }

  agendar(ms: number, f: () => void): () => void {
    const item = { quando: this.t + ms, f, vivo: true };
    this.fila.push(item);
    return () => {
      item.vivo = false;
    };
  }

  /** Avança e dispara, em ordem, o que venceu. */
  avancar(ms: number): void {
    this.t += ms;
    for (;;) {
      const vencidos = this.fila.filter((i) => i.vivo && i.quando <= this.t).sort((a, b) => a.quando - b.quando);
      const proximo = vencidos[0];
      if (!proximo) break;
      proximo.vivo = false;
      proximo.f();
    }
  }

  /** Prazos agendados e ainda vivos (para conferir o orçamento). */
  pendentes(): number[] {
    return this.fila.filter((i) => i.vivo).map((i) => i.quando - this.t);
  }
}

export function armazenamentoEmMemoria(inicial: Record<string, unknown> = {}): Armazenamento & { dados: Record<string, unknown> } {
  const dados: Record<string, unknown> = structuredClone(inicial);
  return {
    dados,
    async ler(chave) {
      return structuredClone(dados[chave]);
    },
    async gravar(chave, valor) {
      dados[chave] = structuredClone(valor);
    },
  };
}

/** Espera as promessas pendentes andarem. */
export async function drenar(vezes = 20): Promise<void> {
  for (let i = 0; i < vezes; i += 1) await Promise.resolve();
}

export interface PedidoRecebido {
  v: number;
  id: string;
  op: string;
  origem: string;
  dados?: Record<string, unknown>;
}

/**
 * Um programa falso atrás de uma porta: `responder` decide, por pedido, o que ele devolve (`null` é
 * não responder). Guarda as portas abertas e o que cada uma recebeu.
 */
export class ProgramaFalso {
  portas: { recebidos: PedidoRecebido[]; desligada: boolean }[] = [];
  private ouvintes: { receber?: (m: unknown) => void; desligar?: (erro: string | undefined) => void }[] = [];

  constructor(public responder: (p: PedidoRecebido) => unknown | null) {}

  conectar = (): PortaNativa => {
    const estado = { recebidos: [] as PedidoRecebido[], desligada: false };
    const ouvinte: { receber?: (m: unknown) => void; desligar?: (erro: string | undefined) => void } = {};
    this.portas.push(estado);
    this.ouvintes.push(ouvinte);
    return {
      enviar: (m) => {
        const p = m as PedidoRecebido;
        estado.recebidos.push(p);
        const r = this.responder(p);
        if (r !== null) queueMicrotask(() => ouvinte.receber?.(r));
      },
      aoReceber: (f) => {
        ouvinte.receber = f;
      },
      aoDesligar: (f) => {
        ouvinte.desligar = f;
      },
      desligar: () => {
        estado.desligada = true;
      },
    };
  };

  /** Entrega uma mensagem pela porta, fora da vez (resposta atrasada, lixo). */
  entregar(indice: number, mensagem: unknown): void {
    this.ouvintes[indice]?.receber?.(mensagem);
  }

  /** O programa sai (ou o navegador não o acha), com a mensagem do navegador. */
  cair(indice: number, erro: string | undefined): void {
    this.ouvintes[indice]?.desligar?.(erro);
  }

  todosOsPedidos(): PedidoRecebido[] {
    return this.portas.flatMap((p) => p.recebidos);
  }
}

export function ok(p: PedidoRecebido, dados: Record<string, unknown>): unknown {
  return { v: 1, id: p.id, ok: true, dados };
}

export function erro(p: PedidoRecebido, codigo: string, detalhe?: unknown): unknown {
  return { v: 1, id: p.id, ok: false, erro: detalhe === undefined ? { codigo } : { codigo, detalhe } };
}

/** Janelas roteirizadas: cada `esperar` fica registrada e o teste decide o desfecho. */
export class JanelasFalsas implements Janelas {
  abertas: { pagina: string; dados: unknown; prazoMs: number; resolver: (d: Desfecho<unknown>) => void }[] = [];

  esperar<T>(pagina: string, _largura: number, _altura: number, dados: unknown, prazoMs: number): Promise<Desfecho<T>> {
    return new Promise((resolver) => {
      this.abertas.push({ pagina, dados, prazoMs, resolver: resolver as (d: Desfecho<unknown>) => void });
    });
  }

  dadosDaJanela(): DadosDaJanela {
    return { estado: 'ausente' };
  }

  decidir(): boolean {
    return false;
  }

  /** Resolve a última janela aberta com a página dada. */
  responder(pagina: string, d: Desfecho<unknown>): void {
    const j = [...this.abertas].reverse().find((a) => a.pagina === pagina);
    if (!j) throw new Error(`nenhuma janela ${pagina} aberta`);
    this.abertas.splice(this.abertas.indexOf(j), 1);
    j.resolver(d);
  }
}
