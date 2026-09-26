import { describe, expect, it } from 'vitest';

import { abrirPrograma, lerResposta, programaAusente } from '../src/nativo';
import { drenar, ok, ProgramaFalso, RelogioManual } from './apoio';

let seq = 0;
const novoId = () => `id-${(seq += 1)}`;

describe('lerResposta', () => {
  it('aceita a resposta do pedido corrente, com dados ou com código do protocolo', () => {
    expect(lerResposta({ v: 1, id: 'a', ok: true, dados: { x: 1 } }, 'a')).toEqual({ ok: true, dados: { x: 1 } });
    expect(lerResposta({ v: 1, id: 'a', ok: false, erro: { codigo: 'pin-incorreto', detalhe: { tentativas: 'ultima' } } }, 'a')).toEqual({
      ok: false,
      erro: { codigo: 'pin-incorreto', detalhe: { tentativas: 'ultima' } },
    });
  });

  it('id de outro pedido, versão errada, código fora do protocolo ou sem forma são `protocolo`', () => {
    for (const m of [
      { v: 1, id: 'b', ok: true, dados: {} },
      { v: 2, id: 'a', ok: true, dados: {} },
      { v: 1, id: 'a', ok: false, erro: { codigo: 'inventado' } },
      { v: 1, id: 'a', ok: true, dados: [] },
      { v: 1, id: 'a' },
      null,
    ]) {
      expect(lerResposta(m, 'a')).toEqual({ ok: false, erro: { codigo: 'protocolo', detalhe: 'resposta do programa sem forma' } });
    }
  });
});

describe('programaAusente', () => {
  it('reconhece as mensagens do Chrome, do Edge e do Firefox', () => {
    expect(programaAusente('Specified native messaging host not found.')).toBe(true);
    expect(programaAusente('No such native application br.com.confidata.assinador')).toBe(true);
    expect(programaAusente('Native host has exited.')).toBe(false);
    expect(programaAusente(undefined)).toBe(false);
  });
});

describe('abrirPrograma', () => {
  it('manda o pedido com a origem e devolve a resposta; a porta só abre no primeiro pedido', async () => {
    const f = new ProgramaFalso((p) => ok(p, { versao: '1.0.0' }));
    const programa = abrirPrograma(f.conectar, novoId, new RelogioManual());
    expect(f.portas).toHaveLength(0);
    const r = await programa.pedir('ola', 'https://demot.confidata.app');
    expect(r).toEqual({ ok: true, dados: { versao: '1.0.0' } });
    expect(f.portas).toHaveLength(1);
    expect(f.portas[0]?.recebidos[0]).toMatchObject({ v: 1, op: 'ola', origem: 'https://demot.confidata.app' });
    expect(f.portas[0]?.recebidos[0]).not.toHaveProperty('dados');
    programa.fechar();
    expect(f.portas[0]?.desligada).toBe(true);
  });

  it('o prazo é o menor entre o teto da operação e o que resta do fluxo', async () => {
    const relogio = new RelogioManual();
    const f = new ProgramaFalso(() => null);
    const programa = abrirPrograma(f.conectar, novoId, relogio);
    const r = programa.pedir('listar', 'https://demot.confidata.app', undefined, 5_000);
    expect(relogio.pendentes()).toEqual([5_000]);
    relogio.avancar(5_000);
    expect(await r).toEqual({ ok: false, erro: { codigo: 'tempo-esgotado', detalhe: 'o programa não respondeu listar' } });
    const semOrcamento = programa.pedir('listar', 'https://demot.confidata.app');
    expect(relogio.pendentes()).toEqual([28_000]);
    relogio.avancar(28_000);
    await semOrcamento;
    expect(await programa.pedir('listar', 'https://demot.confidata.app', undefined, 0)).toMatchObject({ ok: false, erro: { codigo: 'tempo-esgotado' } });
  });

  it('um pedido por vez: o segundo é `ocupado`', async () => {
    const f = new ProgramaFalso(() => null);
    const programa = abrirPrograma(f.conectar, novoId, new RelogioManual());
    void programa.pedir('listar', 'https://demot.confidata.app');
    expect(await programa.pedir('listar', 'https://demot.confidata.app')).toMatchObject({ ok: false, erro: { codigo: 'ocupado' } });
  });

  it('programa não instalado é `nativo-ausente`; processo que morre é `modulo-falhou`, e o pedido seguinte sabe', async () => {
    const f = new ProgramaFalso(() => null);
    const a = abrirPrograma(f.conectar, novoId, new RelogioManual());
    const r = a.pedir('ola', 'https://demot.confidata.app');
    f.cair(0, 'Specified native messaging host not found.');
    expect(await r).toMatchObject({ ok: false, erro: { codigo: 'nativo-ausente' } });
    expect(await a.pedir('ola', 'https://demot.confidata.app')).toMatchObject({ ok: false, erro: { codigo: 'nativo-ausente' } });
    expect(f.portas).toHaveLength(1);

    const b = abrirPrograma(f.conectar, novoId, new RelogioManual());
    const r2 = b.pedir('listar', 'https://demot.confidata.app');
    f.cair(1, undefined);
    expect(await r2).toEqual({ ok: false, erro: { codigo: 'modulo-falhou', detalhe: 'o programa saiu' } });
  });

  it('fechar com pedido pendente resolve como `cancelado`', async () => {
    const f = new ProgramaFalso(() => null);
    const programa = abrirPrograma(f.conectar, novoId, new RelogioManual());
    const r = programa.pedir('assinar', 'https://demot.confidata.app', { ref: 'r' });
    programa.fechar();
    expect(await r).toMatchObject({ ok: false, erro: { codigo: 'cancelado' } });
  });

  it('conexão que lança é `nativo-ausente`', async () => {
    const programa = abrirPrograma(
      () => {
        throw new Error('Access to the specified native messaging host is forbidden.');
      },
      novoId,
      new RelogioManual(),
    );
    expect(await programa.pedir('ola', 'https://demot.confidata.app')).toMatchObject({ ok: false, erro: { codigo: 'nativo-ausente' } });
  });

  it('prazo vencido descarta a porta: a resposta atrasada não chega ao pedido seguinte', async () => {
    const relogio = new RelogioManual();
    let responde = false;
    const f = new ProgramaFalso((p) => (responde ? ok(p, { certificados: [] }) : null));
    const programa = abrirPrograma(f.conectar, novoId, relogio);
    const primeiro = programa.pedir('ola', 'https://demot.confidata.app');
    relogio.avancar(1_200);
    expect(await primeiro).toMatchObject({ ok: false, erro: { codigo: 'tempo-esgotado' } });
    expect(f.portas[0]?.desligada).toBe(true);
    const idVencido = f.portas[0]?.recebidos[0]?.id;
    responde = true;
    const segundo = programa.pedir('listar', 'https://demot.confidata.app');
    // A porta velha fala e cai depois de descartada: nada disso chega ao pedido novo.
    f.entregar(0, { v: 1, id: idVencido, ok: true, dados: { versao: '1.0.0' } });
    f.cair(0, 'Native host has exited.');
    await drenar();
    expect(await segundo).toEqual({ ok: true, dados: { certificados: [] } });
    expect(f.portas).toHaveLength(2);
  });

  it('resposta com o id de outro pedido é `protocolo`, nunca os dados do outro', async () => {
    const f = new ProgramaFalso(() => null);
    const programa = abrirPrograma(f.conectar, novoId, new RelogioManual());
    const r = programa.pedir('listar', 'https://demot.confidata.app');
    f.entregar(0, { v: 1, id: 'outro', ok: true, dados: { certificados: ['de outro'] } });
    expect(await r).toMatchObject({ ok: false, erro: { codigo: 'protocolo' } });
  });
});
