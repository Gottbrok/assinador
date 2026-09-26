import { describe, expect, it } from 'vitest';

import { chaveDoAvisoDoPin, ehVerificacao, lerConfirmacao, textoDaFinalidade } from '../src/confirmar';
import { INTERVALO_DE_DADOS_MS, lerFluxo, pedirDados, TENTATIVAS_DE_DADOS, enviarDecisao } from '../src/janela';
import { lerDiagnostico, linhaDoPrograma } from '../src/opcoes';
import { lerHost } from '../src/permitir';

const t = (chave: string, s?: string | string[]) => (s === undefined ? chave : `${chave}(${([] as string[]).concat(s).join(',')})`);

describe('janela: fluxo e dados', () => {
  it('lê o fluxo da URL e recusa o que não tem forma', () => {
    expect(lerFluxo('?fluxo=0f8c1c2e-1b3a-4c5d-9e6f-a1b2c3d4e5f6')).toBe('0f8c1c2e-1b3a-4c5d-9e6f-a1b2c3d4e5f6');
    expect(lerFluxo('?fluxo=<script>')).toBeNull();
    expect(lerFluxo('')).toBeNull();
  });

  it('repete enquanto o fundo diz `aguarde`, e desiste no limite', async () => {
    const dormidas: number[] = [];
    const dormir = async (ms: number) => {
      dormidas.push(ms);
    };
    let vez = 0;
    const pronto = await pedirDados(async () => (++vez < 3 ? { estado: 'aguarde' } : { estado: 'pronto', dados: { host: 'h' } }), 'f', dormir);
    expect(pronto).toEqual({ host: 'h' });
    expect(dormidas).toEqual([INTERVALO_DE_DADOS_MS, INTERVALO_DE_DADOS_MS]);

    dormidas.length = 0;
    expect(await pedirDados(async () => ({ estado: 'aguarde' }), 'f', dormir)).toBeNull();
    expect(dormidas).toHaveLength(TENTATIVAS_DE_DADOS);
  });

  it('`ausente`, resposta sem forma e fundo que falha são null', async () => {
    const dormir = async () => undefined;
    expect(await pedirDados(async () => ({ estado: 'ausente' }), 'f', dormir)).toBeNull();
    expect(await pedirDados(async () => null, 'f', dormir)).toBeNull();
    expect(
      await pedirDados(
        async () => {
          throw new Error('fundo reiniciado');
        },
        'f',
        dormir,
      ),
    ).toBeNull();
  });

  it('a decisão só conta quando o fundo a aceita', async () => {
    expect(await enviarDecisao(async () => true, 'f', {})).toBe(true);
    expect(await enviarDecisao(async () => false, 'f', {})).toBe(false);
    expect(await enviarDecisao(async () => 'true', 'f', {})).toBe(false);
  });
});

describe('confirmar.html', () => {
  const dados = {
    host: 'demot.confidata.app',
    organizacao: 'Org',
    documento: 'Doc',
    finalidade: 'assinatura',
    certificado: { titular: 'FULANA', emissor: 'AC', validoAte: '2027-01-01T00:00:00Z' },
    exigePin: true,
    estadoDoPin: null,
  };

  it('confere de novo os dados que vieram do fundo', () => {
    expect(lerConfirmacao(dados)).toEqual(dados);
    expect(lerConfirmacao({ ...dados, exigePin: 'sim' })).toBeNull();
    expect(lerConfirmacao({ ...dados, certificado: null })).toBeNull();
    expect(lerConfirmacao({ ...dados, estadoDoPin: 3 })).toBeNull();
    expect(lerConfirmacao(null)).toBeNull();
  });

  it('finalidade e aviso de tentativas', () => {
    expect(textoDaFinalidade('assinatura', t)).toBe('finalidadeAssinatura');
    expect(textoDaFinalidade('verificacao', t)).toBe('finalidadeVerificacao');
    expect(textoDaFinalidade('outra', t)).toBe('outra');
    expect(ehVerificacao('verificacao')).toBe(true);
    expect(ehVerificacao('assinatura')).toBe(false);
    expect(chaveDoAvisoDoPin('poucas-tentativas')).toBe('avisoPoucasTentativas');
    expect(chaveDoAvisoDoPin('ultima-tentativa')).toBe('avisoUltimaTentativa');
    expect(chaveDoAvisoDoPin('ok')).toBeNull();
    expect(chaveDoAvisoDoPin(null)).toBeNull();
  });
});

describe('permitir.html', () => {
  it('lê o host, e só ele', () => {
    expect(lerHost({ host: 'demot.confidata.app' })).toBe('demot.confidata.app');
    expect(lerHost({ host: '' })).toBeNull();
    expect(lerHost({ host: 'a'.repeat(257) })).toBeNull();
    expect(lerHost('demot.confidata.app')).toBeNull();
  });
});

describe('opcoes.html', () => {
  it('a linha do programa diz a versão, a ausência ou o silêncio', () => {
    expect(linhaDoPrograma({ extensao: { versao: '1.0.0' }, nativo: { versao: '1.0.0', protocolo: 1, plataforma: 'linux-amd64' } }, t)).toBe('programaVersao(1.0.0,linux-amd64)');
    expect(linhaDoPrograma({ extensao: { versao: '1.0.0' }, nativo: null, motivoNativo: 'ausente' }, t)).toBe('programaAusente');
    expect(linhaDoPrograma({ extensao: { versao: '1.0.0' }, nativo: null, motivoNativo: 'falhou' }, t)).toBe('programaSemResposta');
    expect(linhaDoPrograma(null, t)).toBe('programaSemResposta');
  });

  it('o diagnóstico devolve o texto ou o código do erro', () => {
    expect(lerDiagnostico({ ok: true, dados: { relatorio: {}, texto: 'oi' } })).toEqual({ texto: 'oi' });
    expect(lerDiagnostico({ ok: false, erro: { codigo: 'nativo-ausente' } })).toEqual({ codigo: 'nativo-ausente' });
    expect(lerDiagnostico(null)).toEqual({ codigo: 'interno' });
  });
});
