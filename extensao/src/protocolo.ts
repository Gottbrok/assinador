/**
 * O vocabulário do protocolo, do lado da extensão.
 *
 * O que as fixtures da biblioteca `@confidata/icp-brasil` definem (a versão do protocolo da página, os
 * códigos de erro e os padrões de origem por emissor) é LIDO da cópia em
 * `protocolo/fixtures/bilhete/protocolo.json`, a mesma que o programa em Go confere: as três pontas
 * não divergem, e mudar o protocolo é mudar a biblioteca primeiro (regra 6 do CLAUDE.md). O que a
 * fixture não traz (os canais, os prazos e o teto da mensagem da página) está espelhado aqui, com a
 * origem dita em cada constante; o repositório é público e não pode depender da biblioteca, que é
 * privada.
 */

import fixture from '../../protocolo/fixtures/bilhete/protocolo.json';

/** `CANAL_PEDIDO` e `CANAL_RESPOSTA` de `assinadorProtocolo.ts` da biblioteca. */
export const CANAL_PEDIDO = 'assinador:pedido';
export const CANAL_RESPOSTA = 'assinador:resposta';

/** O `v` de cada mensagem da página e do programa. */
export const PROTOCOLO: number = fixture.protocoloDaPagina;

export const OPERACOES_DA_PAGINA = ['ola', 'listar', 'assinar', 'diagnostico'] as const;
export type OperacaoDaPagina = (typeof OPERACOES_DA_PAGINA)[number];

/** Operações da extensão para o programa: as da página e o `conferir`, que a página não vê. */
export type OperacaoDoPrograma = OperacaoDaPagina | 'conferir';

/** `TAMANHO_MAXIMO_DA_MENSAGEM` da biblioteca: a extensão recusa o que passar disso. */
export const TAMANHO_MAXIMO_DA_MENSAGEM = 64 * 1024;

/** `TAMANHO_MAXIMO_DO_BILHETE` da biblioteca (e da fixture). */
export const TAMANHO_MAXIMO_DO_BILHETE: number = fixture.tamanhoMaximoDoBilhete;

export const CODIGOS_DE_ERRO: readonly string[] = Object.freeze([...fixture.codigosDeErro]);

export type CodigoDeErro =
  | 'origem-recusada'
  | 'bilhete-invalido'
  | 'bilhete-expirado'
  | 'relogio'
  | 'digest-divergente'
  | 'certificado-divergente'
  | 'certificado-nao-encontrado'
  | 'chave-ausente'
  | 'algoritmo-nao-suportado'
  | 'permissao-negada'
  | 'pin-incorreto'
  | 'token-bloqueado'
  | 'cancelado'
  | 'tempo-esgotado'
  | 'ocupado'
  | 'nativo-ausente'
  | 'nativo-desatualizado'
  | 'modulo-falhou'
  | 'protocolo'
  | 'interno';

export function ehCodigoDeErro(valor: unknown): valor is CodigoDeErro {
  return typeof valor === 'string' && CODIGOS_DE_ERRO.includes(valor);
}

/**
 * O ORÇAMENTO de cada operação da página dentro da extensão, do pedido à resposta, em
 * milissegundos. Fica ABAIXO do prazo da página (`PRAZOS_MS` da biblioteca: `ola` 1,5 s, `listar` e
 * `diagnostico` 30 s, `assinar` 240 s), para a página receber o `tempo-esgotado` da extensão em vez
 * de desistir sem saber por quê, e, pior, com a janela ainda aberta. Cada passo do fluxo (a janela de
 * permissão, o `conferir`, a janela de confirmação, o pedido ao programa) usa o MENOR entre o teto
 * dele e o que resta do orçamento: somados, os tetos passariam do prazo da página (25 s de
 * permissão mais 28 s de `listar`; 28 s de `conferir` mais 180 s de janela mais 100 s de `assinar`).
 */
export const PRAZOS_DO_FLUXO_MS: Readonly<Record<OperacaoDaPagina, number>> = Object.freeze({
  ola: 1_200,
  listar: 29_000,
  diagnostico: 29_000,
  assinar: 235_000,
});

/**
 * O teto de cada pedido ao programa. O `assinar` leva o prazo do filho do módulo (90 s: com leitora
 * de teclado, a pessoa digita o PIN nele) e a folga. Pedido sem orçamento de página (as opções da
 * extensão) usa só o teto.
 */
export const PRAZOS_DO_PROGRAMA_MS: Readonly<Record<OperacaoDoPrograma, number>> = Object.freeze({
  ola: 1_200,
  listar: 28_000,
  diagnostico: 28_000,
  conferir: 28_000,
  assinar: 100_000,
});

/** O teto da janela de confirmação (§3.6 do plano: 180 s sem decisão é `tempo-esgotado`). */
export const PRAZO_DA_DECISAO_MS = 180_000;

/** O teto da janela de permissão. */
export const PRAZO_DA_PERMISSAO_MS = 25_000;

/**
 * O que o orçamento guarda, ao abrir uma janela, para o passo seguinte no programa: a janela de
 * confirmação fecha antes, para sobrar tempo de o cartão assinar (e de a pessoa digitar o PIN na
 * leitora, quando é lá que ele se digita); a de permissão, para o `listar` pelo menos começar.
 */
export const RESERVA_DEPOIS_DA_CONFIRMACAO_MS = 30_000;
export const RESERVA_DEPOIS_DA_PERMISSAO_MS = 3_000;

/** Forma do `id` de uma mensagem: a mesma que o programa exige (`^[A-Za-z0-9_-]{1,64}$`). */
export const FORMA_DO_ID = /^[A-Za-z0-9_-]{1,64}$/;

/** Forma do `ref` e do `digest`: SHA-256 em hexadecimal minúsculo. */
export const FORMA_HEX64 = /^[0-9a-f]{64}$/;

/** Forma do bilhete (JWS compacto): quem o confere é o programa, a extensão só recusa o que nem forma tem. */
export const FORMA_DO_BILHETE = /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/;

export const PADROES_DE_ORIGEM: Readonly<Record<string, readonly string[]>> = Object.freeze(fixture.padroesDeOrigem);
export const PADROES_DE_ORIGEM_DEV: readonly string[] = Object.freeze([...fixture.padroesDeOrigemDev]);

/**
 * A origem que a extensão informa ao programa quando quem pergunta é ela mesma (a página de opções
 * pergunta as versões). O programa responde `ola` a qualquer origem com forma; `.invalid` é reservado
 * e nunca é origem de página de verdade.
 */
export const ORIGEM_DA_EXTENSAO = 'https://extensao.invalid';

export interface ErroDoProtocolo {
  codigo: CodigoDeErro;
  detalhe?: unknown;
}

export type RespostaDoPrograma = { v: number; id: string; ok: true; dados: Record<string, unknown> } | { v: number; id: string; ok: false; erro: ErroDoProtocolo };

/** O que a extensão devolve à página por operação (o `dados` da resposta ou o `erro`). */
export type Resultado = { ok: true; dados: Record<string, unknown> } | { ok: false; erro: ErroDoProtocolo };

export function falha(codigo: CodigoDeErro, detalhe?: string): Resultado {
  return { ok: false, erro: detalhe === undefined ? { codigo } : { codigo, detalhe } };
}
