/**
 * A fila do DISPOSITIVO: os pedidos que carregam os programas do cartão (`listar`, `diagnostico`)
 * rodam um de cada vez, e os que esperam têm teto (`FILA_DO_DISPOSITIVO`). Sem ela, um script numa
 * página permitida abriria um processo do programa por pedido, cada um carregando o middleware do
 * fabricante contra o mesmo cartão.
 *
 * Quem espera na fila espera DENTRO do orçamento dele: o prazo vence na fila e a resposta é
 * `tempo-esgotado`, e a tarefa, quando roda, calcula o que resta do mesmo orçamento.
 */

import type { Relogio } from './nativo';
import { falha, type Resultado } from './protocolo';

export interface Fila {
  executar(prazoMs: number, tarefa: () => Promise<Resultado>): Promise<Resultado>;
}

export function criarFila(relogio: Relogio, maximoEsperando: number): Fila {
  let ocupada = false;
  const esperando: { iniciar: () => void }[] = [];

  const proximo = () => {
    const seguinte = esperando.shift();
    if (seguinte) seguinte.iniciar();
    else ocupada = false;
  };

  const rodar = async (tarefa: () => Promise<Resultado>): Promise<Resultado> => {
    try {
      return await tarefa();
    } catch (erro) {
      return falha('interno', erro instanceof Error ? erro.message : String(erro));
    } finally {
      proximo();
    }
  };

  return {
    executar(prazoMs, tarefa) {
      if (!ocupada) {
        ocupada = true;
        return rodar(tarefa);
      }
      if (esperando.length >= maximoEsperando) return Promise.resolve(falha('ocupado', 'fila do dispositivo cheia'));
      return new Promise<Resultado>((resolver) => {
        const item = {
          iniciar: () => {
            cancelarPrazo();
            resolver(rodar(tarefa));
          },
        };
        const cancelarPrazo = relogio.agendar(prazoMs, () => {
          const i = esperando.indexOf(item);
          if (i < 0) return;
          esperando.splice(i, 1);
          resolver(falha('tempo-esgotado', 'o dispositivo esteve ocupado o prazo inteiro'));
        });
        esperando.push(item);
      });
    },
  };
}
