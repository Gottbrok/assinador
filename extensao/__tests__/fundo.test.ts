import { describe, expect, it } from 'vitest';

import { criarFundo, daPropriaExtensao, lerConferir, lerDadosDoAssinar, lerDecisao, prazoDoPasso, ROTULOS_DO_DIAGNOSTICO, titularDoAssunto, type AmbienteDoFundo } from '../src/fundo';
import type { Remetente } from '../src/origem';
import { listarPermissoes, PREFIXO_DA_PERMISSAO } from '../src/permissoes';
import { EMBARGO_DA_CONFIRMACAO, EMBARGO_DA_PERMISSAO, FILA_DO_DISPOSITIVO, VALIDADE_DO_OLA_MS } from '../src/protocolo';
import { armazenamentoEmMemoria, drenar, erro, JanelasFalsas, ok, ProgramaFalso, RelogioManual, type PedidoRecebido } from './apoio';

const ID = 'jmogljhnfdnhhoapijclifkpjfbhhppf';
const RAIZ = `chrome-extension://${ID}/`;
const ORIGEM = 'https://demot.confidata.app';
const HEX = (c: string) => c.repeat(64);
const BILHETE = 'eyJhbGciOiJFUzI1NiJ9.eyJ2IjoxfQ.YXNzaW5hdHVyYQ';
const DADOS_DE_ASSINAR = { ref: HEX('a'), digest: HEX('b'), bilhete: BILHETE };

const CONFERIDO = {
  emissor: 'confidata',
  organizacao: 'Organização Demo',
  documento: 'Contrato de prestação de serviços',
  finalidade: 'assinatura',
  expiraEm: '2026-09-26T12:05:00Z',
  certificado: { assunto: 'FULANA DE TAL:***********', emissor: 'AC TESTE', validoAte: '2027-09-26T00:00:00Z', exigePin: true, estadoDoPin: 'ok' },
};

/** O programa padrão: responde tudo com sucesso. */
function programaPadrao(p: PedidoRecebido): unknown {
  switch (p.op) {
    case 'ola':
      return ok(p, { versao: '1.0.0', protocolo: 1, plataforma: 'linux-amd64' });
    case 'listar':
      return ok(p, { certificados: [{ ref: HEX('a') }], avisos: [] });
    case 'diagnostico':
      return ok(p, { relatorio: { leitoras: [] }, texto: 'Leitoras: nenhuma' });
    case 'conferir':
      return ok(p, CONFERIDO);
    case 'assinar':
      return ok(p, { assinatura: 'QUJD' });
    default:
      return erro(p, 'protocolo');
  }
}

function montar(opcoes: { dev?: boolean; permitidas?: string[]; responder?: (p: PedidoRecebido) => unknown | null; diagnosticoParaPaginas?: boolean } = {}) {
  const programa = new ProgramaFalso(opcoes.responder ?? programaPadrao);
  const janelas = new JanelasFalsas();
  const relogio = new RelogioManual();
  const armazenamento = armazenamentoEmMemoria(
    Object.fromEntries((opcoes.permitidas ?? []).map((o) => [`${PREFIXO_DA_PERMISSAO}${o}`, { desde: '2026-09-01T00:00:00.000Z' }])),
  );
  let seq = 0;
  const amb: AmbienteDoFundo = {
    idDaExtensao: ID,
    urlDaExtensao: RAIZ,
    versao: '1.0.0',
    dev: opcoes.dev ?? false,
    navegador: 'Navegador de teste',
    conectar: programa.conectar,
    armazenamento,
    janelas,
    novoId: () => `id-${(seq += 1)}`,
    relogio,
    podeEntregarDiagnostico: async () => opcoes.diagnosticoParaPaginas ?? true,
  };
  return { fundo: criarFundo(amb), programa, janelas, relogio, armazenamento };
}

/** Uma saída de página que o teste dispara. */
function saidaDaPagina() {
  let sair: () => void = () => undefined;
  const saiu = new Promise<void>((r) => {
    sair = r;
  });
  return { saiu, sair };
}

const daPagina = (origem = ORIGEM, extra: Partial<Remetente> = {}): Remetente => ({ id: ID, origin: origem, url: `${origem}/assinar/x`, frameId: 0, tab: { id: 7, windowId: 1 }, ...extra });
const pedido = (op: string, dados?: unknown, origem = ORIGEM) => ({ tipo: 'pedido-da-pagina', id: 'p1', op, origem, ...(dados === undefined ? {} : { dados }) });

describe('portões do fundo', () => {
  it('recusa remetente que não é o script de conteúdo desta extensão no quadro de topo de uma aba', async () => {
    const { fundo, programa } = montar({ permitidas: [ORIGEM] });
    for (const r of [daPagina(ORIGEM, { id: 'outra' }), daPagina(ORIGEM, { frameId: 3 }), daPagina(ORIGEM, { tab: undefined })]) {
      expect(await fundo.atenderPagina(pedido('listar'), r)).toEqual({ ok: false, erro: { codigo: 'origem-recusada', detalhe: 'remetente' } });
    }
    expect(programa.portas).toHaveLength(0);
  });

  it('a origem é a que o NAVEGADOR diz, igual à declarada e nos padrões', async () => {
    const { fundo, programa } = montar({ permitidas: [ORIGEM, 'https://evil.com'] });
    // Declara uma origem e o navegador diz outra.
    expect(await fundo.atenderPagina(pedido('listar', undefined, ORIGEM), daPagina('https://outra.confidata.app'))).toMatchObject({ erro: { codigo: 'origem-recusada' } });
    // Origem fora dos padrões, mesmo com permissão gravada.
    expect(await fundo.atenderPagina(pedido('listar', undefined, 'https://evil.com'), daPagina('https://evil.com'))).toMatchObject({ erro: { codigo: 'origem-recusada' } });
    expect(programa.portas).toHaveLength(0);
  });

  it('no Firefox, sem sender.origin, vale a origem de sender.url', async () => {
    const { fundo } = montar({ permitidas: [ORIGEM] });
    const firefox: Remetente = { id: ID, url: `${ORIGEM}/assinar/x`, frameId: 0, tab: { id: 7 } };
    expect(await fundo.atenderPagina(pedido('listar'), firefox)).toMatchObject({ ok: true });
  });

  it('localhost só no build de desenvolvimento', async () => {
    const local = 'http://localhost:3000';
    const release = montar({ permitidas: [local] });
    expect(await release.fundo.atenderPagina(pedido('listar', undefined, local), daPagina(local))).toMatchObject({ erro: { codigo: 'origem-recusada' } });
    const dev = montar({ permitidas: [local], dev: true });
    expect(await dev.fundo.atenderPagina(pedido('listar', undefined, local), daPagina(local))).toMatchObject({ ok: true });
  });

  it('mensagem sem forma é `protocolo`, e operação desconhecida também', async () => {
    const { fundo } = montar({ permitidas: [ORIGEM] });
    expect(await fundo.atenderPagina('lixo', daPagina())).toMatchObject({ erro: { codigo: 'protocolo' } });
    expect(await fundo.atenderPagina(pedido('conferir'), daPagina())).toMatchObject({ erro: { codigo: 'protocolo', detalhe: 'operação desconhecida' } });
    expect(await fundo.atenderPagina(pedido('formatar'), daPagina())).toMatchObject({ erro: { codigo: 'protocolo' } });
  });
});

describe('ola', () => {
  it('não pede permissão e diz as duas versões', async () => {
    const { fundo, janelas, programa } = montar();
    expect(await fundo.atenderPagina(pedido('ola'), daPagina())).toEqual({
      ok: true,
      dados: { extensao: { versao: '1.0.0' }, nativo: { versao: '1.0.0', protocolo: 1, plataforma: 'linux-amd64' } },
    });
    expect(janelas.abertas).toHaveLength(0);
    expect(programa.portas[0]?.desligada).toBe(true);
  });

  it('sem o programa, responde `nativo: null` com o motivo, nunca erro', async () => {
    const ausente = montar({ responder: () => null });
    const r = ausente.fundo.atenderPagina(pedido('ola'), daPagina());
    await drenar();
    ausente.programa.cair(0, 'Specified native messaging host not found.');
    expect(await r).toEqual({ ok: true, dados: { extensao: { versao: '1.0.0' }, nativo: null, motivoNativo: 'ausente' } });

    const lento = montar({ responder: () => null });
    const r2 = lento.fundo.atenderPagina(pedido('ola'), daPagina());
    await drenar();
    lento.relogio.avancar(1_200);
    expect(await r2).toEqual({ ok: true, dados: { extensao: { versao: '1.0.0' }, nativo: null, motivoNativo: 'falhou' } });
  });
});

describe('permissão por endereço', () => {
  it('endereço novo abre permitir.html com o host do NAVEGADOR; negar é `permissao-negada` sem falar com o programa', async () => {
    const { fundo, janelas, programa, armazenamento } = montar();
    const r = fundo.atenderPagina(pedido('listar'), daPagina());
    await drenar();
    expect(janelas.abertas).toEqual([expect.objectContaining({ pagina: 'permitir.html', dados: { host: 'demot.confidata.app' } })]);
    janelas.responder('permitir.html', { tipo: 'decisao', valor: { permitir: false } });
    expect(await r).toEqual({ ok: false, erro: { codigo: 'permissao-negada' } });
    expect(programa.portas).toHaveLength(0);
    expect(armazenamento.dados).toEqual({});
  });

  it('permitir grava, lista, e a segunda vez não pergunta; revogado, pergunta de novo', async () => {
    const { fundo, janelas, armazenamento } = montar();
    const r = fundo.atenderPagina(pedido('listar'), daPagina());
    await drenar();
    janelas.responder('permitir.html', { tipo: 'decisao', valor: { permitir: true } });
    expect(await r).toMatchObject({ ok: true, dados: { certificados: [{ ref: HEX('a') }] } });
    expect((await listarPermissoes(armazenamento)).map((p) => p.origem)).toEqual([ORIGEM]);
    expect(await fundo.atenderPagina(pedido('listar'), daPagina())).toMatchObject({ ok: true });
    expect(janelas.abertas).toHaveLength(0);

    await armazenamento.apagar(`${PREFIXO_DA_PERMISSAO}${ORIGEM}`);
    void fundo.atenderPagina(pedido('listar'), daPagina());
    await drenar();
    expect(janelas.abertas).toHaveLength(1);
  });

  it('negar, fechar ou decisão sem forma é `permissao-negada`; o prazo é `tempo-esgotado`, e nada foi negado', async () => {
    for (const [d, codigo] of [
      [{ tipo: 'fechada' as const }, 'permissao-negada'],
      [{ tipo: 'decisao' as const, valor: { permitir: 'sim' } }, 'permissao-negada'],
      [{ tipo: 'prazo' as const }, 'tempo-esgotado'],
      [{ tipo: 'falhou' as const }, 'interno'],
    ] as const) {
      const { fundo, janelas, armazenamento } = montar();
      const r = fundo.atenderPagina(pedido('listar'), daPagina());
      await drenar();
      janelas.responder('permitir.html', d);
      expect(await r, d.tipo).toMatchObject({ erro: { codigo } });
      expect(armazenamento.dados).toEqual({});
    }
  });

  it('página que sai com a janela de permissão aberta fecha a janela e não grava nada', async () => {
    const { fundo, janelas, armazenamento, programa } = montar();
    const { saiu, sair } = saidaDaPagina();
    const r = fundo.atenderPagina(pedido('listar'), daPagina(), saiu);
    await drenar();
    sair();
    expect(await r).toMatchObject({ ok: false, erro: { codigo: 'cancelado' } });
    expect(janelas.interrompidas).toEqual(['permitir.html']);
    expect(armazenamento.dados).toEqual({});
    expect(programa.portas).toHaveLength(0);
  });

  it(`${EMBARGO_DA_PERMISSAO.recusas} recusas seguidas embargam o endereço: nenhuma janela até o embargo passar`, async () => {
    const { fundo, janelas, relogio } = montar();
    for (let i = 0; i < EMBARGO_DA_PERMISSAO.recusas; i += 1) {
      const r = fundo.atenderPagina(pedido('listar'), daPagina());
      await drenar();
      janelas.responder('permitir.html', { tipo: 'fechada' });
      await r;
    }
    expect(await fundo.atenderPagina(pedido('listar'), daPagina())).toMatchObject({ ok: false, erro: { codigo: 'permissao-negada', detalhe: expect.stringContaining('embargo') } });
    expect(janelas.abertas).toHaveLength(0);
    // Outro endereço não herda o embargo.
    void fundo.atenderPagina(pedido('listar', undefined, 'https://outra.confidata.app'), daPagina('https://outra.confidata.app'));
    await drenar();
    expect(janelas.abertas).toHaveLength(1);
    janelas.responder('permitir.html', { tipo: 'fechada' });
    relogio.avancar(EMBARGO_DA_PERMISSAO.duracaoMs);
    void fundo.atenderPagina(pedido('listar'), daPagina());
    await drenar();
    expect(janelas.abertas.map((j) => j.pagina)).toEqual(['permitir.html']);
  });

  it('dois pedidos do mesmo endereço ao mesmo tempo abrem UMA janela', async () => {
    const { fundo, janelas } = montar();
    const a = fundo.atenderPagina(pedido('listar'), daPagina());
    const b = fundo.atenderPagina(pedido('diagnostico'), daPagina());
    await drenar();
    expect(janelas.abertas).toHaveLength(1);
    janelas.responder('permitir.html', { tipo: 'decisao', valor: { permitir: true } });
    expect(await a).toMatchObject({ ok: true });
    expect(await b).toMatchObject({ ok: true });
  });

  it('a janela de permissão cabe no orçamento do listar, com a reserva para o programa', async () => {
    const { fundo, janelas } = montar();
    void fundo.atenderPagina(pedido('listar'), daPagina());
    await drenar();
    expect(janelas.abertas[0]?.prazoMs).toBe(25_000);
    expect(janelas.abertas[0]?.prazoMs).toBeLessThanOrEqual(29_000 - 3_000);
  });
});

describe('diagnóstico da página', () => {
  it('acrescenta a versão da extensão e o navegador ao relatório do programa', async () => {
    const { fundo } = montar({ permitidas: [ORIGEM] });
    expect(await fundo.atenderPagina(pedido('diagnostico'), daPagina())).toEqual({
      ok: true,
      dados: {
        relatorio: { leitoras: [], extensao: { versao: '1.0.0' }, navegador: 'Navegador de teste' },
        texto: `${ROTULOS_DO_DIAGNOSTICO.extensao} 1.0.0\n${ROTULOS_DO_DIAGNOSTICO.navegador} Navegador de teste\nLeitoras: nenhuma`,
      },
    });
  });

  it('sem o consentimento do dado técnico (Firefox), a página não recebe o diagnóstico; as opções recebem', async () => {
    const { fundo, programa } = montar({ permitidas: [ORIGEM], diagnosticoParaPaginas: false });
    expect(await fundo.atenderPagina(pedido('diagnostico'), daPagina())).toMatchObject({ ok: false, erro: { codigo: 'permissao-negada' } });
    expect(programa.portas).toHaveLength(0);
    expect(await fundo.atenderExtensao({ tipo: 'diagnostico' }, { id: ID, url: `${RAIZ}opcoes.html` })).toMatchObject({ ok: true });
  });
});

describe('assinar', () => {
  /** Começa a assinatura e espera a janela abrir. A promessa vem num objeto: `async` achataria a do fluxo. */
  async function ateAJanela(m: ReturnType<typeof montar>, dados: unknown = DADOS_DE_ASSINAR) {
    const fluxo = m.fundo.atenderPagina(pedido('assinar', dados), daPagina());
    await drenar();
    return { fluxo };
  }

  it('conferir, janela com o que o bilhete e o navegador dizem, PIN, e assinar na MESMA porta', async () => {
    const m = montar({ permitidas: [ORIGEM] });
    const { fluxo: r } = await ateAJanela(m);
    const janela = m.janelas.abertas[0];
    expect(janela?.pagina).toBe('confirmar.html');
    expect(janela?.dados).toEqual({
      host: 'demot.confidata.app',
      organizacao: 'Organização Demo',
      documento: 'Contrato de prestação de serviços',
      finalidade: 'assinatura',
      certificado: { titular: 'FULANA DE TAL', emissor: 'AC TESTE', validoAte: '2027-09-26T00:00:00Z' },
      exigePin: true,
      estadoDoPin: 'ok',
    });
    m.janelas.responder('confirmar.html', { tipo: 'decisao', valor: { assinar: true, pin: '123456' } });
    expect(await r).toEqual({ ok: true, dados: { assinatura: 'QUJD' } });
    expect(m.programa.portas).toHaveLength(1);
    const [conferir, assinar] = m.programa.portas[0]?.recebidos ?? [];
    expect(conferir).toMatchObject({ op: 'conferir', origem: ORIGEM, dados: DADOS_DE_ASSINAR });
    expect(assinar).toMatchObject({ op: 'assinar', origem: ORIGEM, dados: { ...DADOS_DE_ASSINAR, pin: '123456' } });
    expect(m.programa.portas[0]?.desligada).toBe(true);
  });

  it('dispositivo sem PIN na janela (leitora com teclado): o PIN que vier não vai ao programa', async () => {
    const m = montar({
      permitidas: [ORIGEM],
      responder: (p) => (p.op === 'conferir' ? ok(p, { ...CONFERIDO, certificado: { ...CONFERIDO.certificado, exigePin: false } }) : programaPadrao(p)),
    });
    const { fluxo: r } = await ateAJanela(m);
    m.janelas.responder('confirmar.html', { tipo: 'decisao', valor: { assinar: true, pin: '999' } });
    expect(await r).toMatchObject({ ok: true });
    expect(m.programa.portas[0]?.recebidos[1]?.dados).toEqual(DADOS_DE_ASSINAR);
  });

  it('dados sem forma são `protocolo` e o programa nem é chamado', async () => {
    for (const dados of [
      undefined,
      { ...DADOS_DE_ASSINAR, pin: '123' },
      { ...DADOS_DE_ASSINAR, ref: 'A'.repeat(64) },
      { ...DADOS_DE_ASSINAR, digest: HEX('b').slice(1) },
      { ...DADOS_DE_ASSINAR, bilhete: 'sem.forma' },
      { ...DADOS_DE_ASSINAR, bilhete: `${'a'.repeat(4096)}.b.c` },
    ]) {
      const m = montar({ permitidas: [ORIGEM] });
      expect(await m.fundo.atenderPagina(pedido('assinar', dados), daPagina())).toMatchObject({ erro: { codigo: 'protocolo' } });
      expect(m.programa.portas).toHaveLength(0);
    }
  });

  it('recusa do conferir (bilhete) chega à página sem janela', async () => {
    const m = montar({ permitidas: [ORIGEM], responder: (p) => (p.op === 'conferir' ? erro(p, 'bilhete-expirado') : programaPadrao(p)) });
    expect(await m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina())).toEqual({ ok: false, erro: { codigo: 'bilhete-expirado' } });
    expect(m.janelas.abertas).toHaveLength(0);
  });

  it('token já bloqueado é `token-bloqueado` antes da janela', async () => {
    const m = montar({
      permitidas: [ORIGEM],
      responder: (p) => (p.op === 'conferir' ? ok(p, { ...CONFERIDO, certificado: { ...CONFERIDO.certificado, estadoDoPin: 'bloqueado' } }) : programaPadrao(p)),
    });
    expect(await m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina())).toMatchObject({ erro: { codigo: 'token-bloqueado' } });
    expect(m.janelas.abertas).toHaveLength(0);
  });

  it('cada desfecho da janela vira o código certo, e o programa não assina', async () => {
    const casos = [
      [{ tipo: 'fechada' }, 'cancelado'],
      [{ tipo: 'prazo' }, 'tempo-esgotado'],
      [{ tipo: 'falhou' }, 'interno'],
      [{ tipo: 'decisao', valor: { assinar: false } }, 'cancelado'],
      [{ tipo: 'decisao', valor: { assinar: true } }, 'protocolo'],
      [{ tipo: 'decisao', valor: 'sim' }, 'protocolo'],
      [{ tipo: 'decisao', valor: { assinar: true, pin: 'a\u0000b' } }, 'protocolo'],
    ] as const;
    for (const [desfecho, codigo] of casos) {
      const m = montar({ permitidas: [ORIGEM] });
      const { fluxo: r } = await ateAJanela(m);
      m.janelas.responder('confirmar.html', desfecho);
      expect(await r, JSON.stringify(desfecho)).toMatchObject({ ok: false, erro: { codigo } });
      expect(m.programa.todosOsPedidos().map((p) => p.op)).toEqual(['conferir']);
      expect(m.programa.portas[0]?.desligada).toBe(true);
    }
  });

  it('uma assinatura por vez: a segunda é `ocupado`, e a trava solta no fim, mesmo na falha', async () => {
    const m = montar({ permitidas: [ORIGEM] });
    const { fluxo: primeira } = await ateAJanela(m);
    expect(await m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina())).toMatchObject({ erro: { codigo: 'ocupado' } });
    m.janelas.responder('confirmar.html', { tipo: 'fechada' });
    expect(await primeira).toMatchObject({ erro: { codigo: 'cancelado' } });
    const { fluxo: segunda } = await ateAJanela(m);
    expect(m.janelas.abertas).toHaveLength(1);
    m.janelas.responder('confirmar.html', { tipo: 'decisao', valor: { assinar: true, pin: '1' } });
    expect(await segunda).toMatchObject({ ok: true });
  });

  it('o orçamento de 235 s vale para o fluxo inteiro: a janela fecha com a reserva para o cartão', async () => {
    let assinarPrazo = -1;
    const m = montar({ permitidas: [ORIGEM] });
    const r = m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina());
    await drenar();
    expect(m.janelas.abertas[0]?.prazoMs).toBe(180_000);
    // A pessoa demora 170 s para decidir.
    m.relogio.avancar(170_000);
    m.programa.responder = (p) => {
      if (p.op === 'assinar') {
        assinarPrazo = Math.max(...m.relogio.pendentes());
        return null;
      }
      return programaPadrao(p);
    };
    m.janelas.responder('confirmar.html', { tipo: 'decisao', valor: { assinar: true, pin: '1' } });
    await drenar();
    // Sobram 65 s do orçamento, abaixo do teto de 100 s do assinar.
    expect(assinarPrazo).toBe(65_000);
    m.relogio.avancar(65_000);
    expect(await r).toMatchObject({ ok: false, erro: { codigo: 'tempo-esgotado' } });
  });

  it('conferir lento ENCURTA a janela: 27 s de conferir deixam 178 s de janela (235 - 27 - 30)', async () => {
    let responderConferir: (() => void) | undefined;
    const m = montar({
      permitidas: [ORIGEM],
      responder: (p) => {
        if (p.op !== 'conferir') return programaPadrao(p);
        responderConferir = () => m.programa.entregar(0, programaPadrao(p));
        return null;
      },
    });
    void m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina());
    await drenar();
    m.relogio.avancar(27_000);
    responderConferir?.();
    await drenar();
    expect(m.janelas.abertas[0]?.prazoMs).toBe(178_000);
  });

  it('conferir que não responde é `tempo-esgotado`, sem janela', async () => {
    const m = montar({ permitidas: [ORIGEM], responder: () => null });
    const r = m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina());
    await drenar();
    m.relogio.avancar(28_000);
    expect(await r).toMatchObject({ ok: false, erro: { codigo: 'tempo-esgotado' } });
    expect(m.janelas.abertas).toHaveLength(0);
  });

  it('a página que sai com a janela aberta fecha a janela, solta a assinatura e o programa não assina', async () => {
    const m = montar({ permitidas: [ORIGEM] });
    const { saiu, sair } = saidaDaPagina();
    const r = m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina(), saiu);
    await drenar();
    expect(m.janelas.abertas).toHaveLength(1);
    sair();
    expect(await r).toMatchObject({ ok: false, erro: { codigo: 'cancelado', detalhe: 'a página saiu' } });
    expect(m.janelas.interrompidas).toEqual(['confirmar.html']);
    expect(m.programa.todosOsPedidos().map((p) => p.op)).toEqual(['conferir']);
    expect(m.programa.portas[0]?.desligada).toBe(true);
    // A assinatura foi solta: outra aba assina.
    void m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina());
    await drenar();
    expect(m.janelas.abertas).toHaveLength(1);
  });

  it('o programa que cai com a janela aberta fecha a janela na hora, com a causa', async () => {
    const m = montar({ permitidas: [ORIGEM] });
    const r = m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina());
    await drenar();
    m.programa.cair(0, 'Native host has exited.');
    expect(await r).toMatchObject({ ok: false, erro: { codigo: 'modulo-falhou' } });
    expect(m.janelas.interrompidas).toEqual(['confirmar.html']);
  });

  it(`${EMBARGO_DA_CONFIRMACAO.recusas} confirmações recusadas seguidas embargam o endereço; assinar zera a conta`, async () => {
    const m = montar({ permitidas: [ORIGEM] });
    const recusar = async () => {
      const r = m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina());
      await drenar();
      m.janelas.responder('confirmar.html', { tipo: 'decisao', valor: { assinar: false } });
      return r;
    };
    // Duas recusas, uma assinatura (zera), duas recusas: ainda não embargado.
    await recusar();
    await recusar();
    const assinada = m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina());
    await drenar();
    m.janelas.responder('confirmar.html', { tipo: 'decisao', valor: { assinar: true, pin: '1' } });
    expect(await assinada).toMatchObject({ ok: true });
    await recusar();
    await recusar();
    expect(m.janelas.abertas).toHaveLength(0);
    await recusar();
    const conferidosAntes = m.programa.todosOsPedidos().length;
    expect(await m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina())).toMatchObject({ ok: false, erro: { codigo: 'cancelado', detalhe: expect.stringContaining('embargo') } });
    expect(m.programa.todosOsPedidos()).toHaveLength(conferidosAntes);
    m.relogio.avancar(EMBARGO_DA_CONFIRMACAO.duracaoMs);
    void m.fundo.atenderPagina(pedido('assinar', DADOS_DE_ASSINAR), daPagina());
    await drenar();
    expect(m.janelas.abertas).toHaveLength(1);
  });
});

describe('processos do programa', () => {
  it('o `ola` é um só em voo e vale por alguns segundos: uma página em laço não abre um processo por pedido', async () => {
    const m = montar();
    const pedidos = Array.from({ length: 20 }, () => m.fundo.atenderPagina(pedido('ola'), daPagina()));
    const respostas = await Promise.all(pedidos);
    expect(new Set(respostas.map((r) => JSON.stringify(r))).size).toBe(1);
    expect(m.programa.portas).toHaveLength(1);
    await m.fundo.atenderPagina(pedido('ola'), daPagina());
    expect(m.programa.portas).toHaveLength(1);
    m.relogio.avancar(VALIDADE_DO_OLA_MS);
    await m.fundo.atenderPagina(pedido('ola'), daPagina());
    expect(m.programa.portas).toHaveLength(2);
  });

  it('`listar` e `diagnostico` vão pela fila do dispositivo: um por vez, e o excesso é `ocupado`', async () => {
    const pendentes: (() => void)[] = [];
    const m = montar({
      permitidas: [ORIGEM],
      responder: (p) => {
        // A porta que acabou de receber é a última aberta: o índice se guarda agora, não na entrega.
        const porta = m.programa.portas.length - 1;
        pendentes.push(() => m.programa.entregar(porta, programaPadrao(p)));
        return null;
      },
    });
    const emVoo = Array.from({ length: FILA_DO_DISPOSITIVO + 1 }, () => m.fundo.atenderPagina(pedido('listar'), daPagina()));
    await drenar();
    // Um processo rodando, os outros esperando na fila.
    expect(m.programa.portas).toHaveLength(1);
    expect(await m.fundo.atenderPagina(pedido('listar'), daPagina())).toMatchObject({ ok: false, erro: { codigo: 'ocupado' } });
    for (let i = 0; i < emVoo.length; i += 1) {
      pendentes.shift()?.();
      await drenar();
    }
    for (const r of await Promise.all(emVoo)) expect(r).toMatchObject({ ok: true });
    expect(m.programa.portas).toHaveLength(FILA_DO_DISPOSITIVO + 1);
    expect(m.programa.portas.every((p) => p.desligada)).toBe(true);
  });

  it('quem espera na fila espera dentro do orçamento: vencido, é `tempo-esgotado` sem abrir o programa', async () => {
    // Três pedidos no mesmo instante: o primeiro roda 28 s (o teto), o segundo roda o 1 s que sobra
    // do orçamento dele, e o terceiro vence ainda na fila.
    const m = montar({ permitidas: [ORIGEM], responder: () => null });
    const pedidos = [0, 1, 2].map(() => m.fundo.atenderPagina(pedido('listar'), daPagina()));
    await drenar();
    m.relogio.avancar(28_000);
    await drenar();
    m.relogio.avancar(1_000);
    await drenar();
    const respostas = await Promise.all(pedidos);
    expect(respostas.every((r) => !r.ok && r.erro.codigo === 'tempo-esgotado')).toBe(true);
    expect(m.programa.todosOsPedidos()).toHaveLength(2);
  });
});

describe('prazoDoPasso', () => {
  it('é o menor entre o teto e o que resta menos a reserva, e pode chegar a zero', () => {
    expect(prazoDoPasso(180_000, 233_000, 30_000)).toBe(180_000);
    expect(prazoDoPasso(180_000, 208_000, 30_000)).toBe(178_000);
    expect(prazoDoPasso(25_000, 20_000, 3_000)).toBe(17_000);
    expect(prazoDoPasso(25_000, 2_000, 3_000)).toBeLessThanOrEqual(0);
  });
});

describe('páginas da própria extensão', () => {
  const daExtensao = (pagina: string): Remetente => ({ id: ID, url: `${RAIZ}${pagina}`, tab: { id: 9 } });

  it('só página desta extensão fala por este canal; o script de conteúdo não', async () => {
    expect(daPropriaExtensao(daExtensao('opcoes.html'), { idDaExtensao: ID, urlDaExtensao: RAIZ })).toBe(true);
    expect(daPropriaExtensao({ id: ID, origin: RAIZ.slice(0, -1) }, { idDaExtensao: ID, urlDaExtensao: RAIZ })).toBe(true);
    expect(daPropriaExtensao(daPagina(), { idDaExtensao: ID, urlDaExtensao: RAIZ })).toBe(false);
    expect(daPropriaExtensao({ id: 'outra', url: `${RAIZ}opcoes.html` }, { idDaExtensao: ID, urlDaExtensao: RAIZ })).toBe(false);
    expect(daPropriaExtensao({ id: ID, url: `chrome-extension://${ID}x/opcoes.html` }, { idDaExtensao: ID, urlDaExtensao: RAIZ })).toBe(false);
    const { fundo } = montar();
    expect(await fundo.atenderExtensao({ tipo: 'versoes' }, daPagina())).toBeNull();
  });

  it('as opções pedem as versões e o diagnóstico com a origem da extensão, sem permissão', async () => {
    const { fundo, programa, janelas } = montar();
    expect(await fundo.atenderExtensao({ tipo: 'versoes' }, daExtensao('opcoes.html'))).toEqual({
      extensao: { versao: '1.0.0' },
      nativo: { versao: '1.0.0', protocolo: 1, plataforma: 'linux-amd64' },
    });
    expect(await fundo.atenderExtensao({ tipo: 'diagnostico' }, daExtensao('opcoes.html'))).toMatchObject({ ok: true, dados: { texto: expect.stringContaining('Leitoras') } });
    expect(programa.todosOsPedidos().map((p) => [p.op, p.origem])).toEqual([
      ['ola', 'https://extensao.invalid'],
      ['diagnostico', 'https://extensao.invalid'],
    ]);
    expect(janelas.abertas).toHaveLength(0);
  });

  it('mensagem desconhecida volta null', async () => {
    const { fundo } = montar();
    expect(await fundo.atenderExtensao({ tipo: 'listar' }, daExtensao('opcoes.html'))).toBeNull();
    expect(await fundo.atenderExtensao('lixo', daExtensao('opcoes.html'))).toBeNull();
  });
});

describe('leitores', () => {
  it('lerDadosDoAssinar é estrito', () => {
    expect(lerDadosDoAssinar(DADOS_DE_ASSINAR)).toEqual(DADOS_DE_ASSINAR);
    expect(lerDadosDoAssinar({ ...DADOS_DE_ASSINAR, extra: 1 })).toBeNull();
    expect(lerDadosDoAssinar({ ref: HEX('a'), digest: HEX('b') })).toBeNull();
    expect(lerDadosDoAssinar([DADOS_DE_ASSINAR])).toBeNull();
  });

  it('titularDoAssunto tira o que vem depois do `:`', () => {
    expect(titularDoAssunto('FULANA DE TAL:***********')).toBe('FULANA DE TAL');
    expect(titularDoAssunto('SEM DOCUMENTO')).toBe('SEM DOCUMENTO');
    expect(titularDoAssunto(':123')).toBe(':123');
  });

  it('lerConferir recusa o que não tem forma', () => {
    expect(lerConferir({ ...CONFERIDO, certificado: { ...CONFERIDO.certificado, exigePin: 'sim' } }, 'h')).toBeNull();
    expect(lerConferir({ ...CONFERIDO, organizacao: 1 }, 'h')).toBeNull();
    expect(lerConferir({ ...CONFERIDO, certificado: { ...CONFERIDO.certificado, estadoDoPin: undefined } }, 'h')?.estadoDoPin).toBeNull();
  });

  it('lerDecisao segue as regras do PIN do programa', () => {
    expect(lerDecisao({ assinar: false, pin: 'ignorado' })).toEqual({ assinar: false });
    expect(lerDecisao({ assinar: true, pin: '1234' })).toEqual({ assinar: true, pin: '1234' });
    expect(lerDecisao({ assinar: true, pin: '' })).toBeNull();
    expect(lerDecisao({ assinar: true, pin: 'é'.repeat(33) })).toBeNull();
    expect(lerDecisao({ assinar: true, pin: 'a'.repeat(64) })).toEqual({ assinar: true, pin: 'a'.repeat(64) });
    expect(lerDecisao({ assinar: true, pin: 'a\u007fb' })).toBeNull();
    expect(lerDecisao({ assinar: 'true' })).toBeNull();
  });
});
