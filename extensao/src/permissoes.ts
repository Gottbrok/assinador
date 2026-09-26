/**
 * Permissão por endereço (§3.3 e §3.6 do plano).
 *
 * `listar`, `assinar` e `diagnostico` só atendem uma origem que a pessoa autorizou nesta extensão.
 * Sem isso, qualquer script que rodasse numa página nossa leria em silêncio o nome e o CPF de todos
 * os certificados do computador (o CN ICP-Brasil é `NOME:CPF`). `ola` não precisa: diz só versões.
 *
 * A decisão fica em `storage.local`, NUNCA no `storage.sync` (permissão num computador não vale para
 * outro), por ORIGEM (esquema, host e porta): `https://demot.confidata.app` autorizado não autoriza
 * `https://outra.confidata.app`. A página de opções lista e revoga uma a uma.
 */

export const CHAVE_DAS_PERMISSOES = 'permissoes';

export interface Permissao {
  origem: string;
  /** Quando a pessoa permitiu (ISO 8601). */
  desde: string;
}

export interface Armazenamento {
  ler(chave: string): Promise<unknown>;
  gravar(chave: string, valor: unknown): Promise<void>;
}

type Mapa = Record<string, { desde: string }>;

function ehObjeto(valor: unknown): valor is Record<string, unknown> {
  return typeof valor === 'object' && valor !== null && !Array.isArray(valor);
}

/** O mapa gravado, só com as entradas que têm forma (o que não tem é ignorado, nunca vira permissão). */
async function lerMapa(armazenamento: Armazenamento): Promise<Mapa> {
  const bruto = await armazenamento.ler(CHAVE_DAS_PERMISSOES);
  // Sem protótipo: nome de propriedade herdada (`toString`, `__proto__`) nunca vira permissão.
  const mapa: Mapa = Object.create(null) as Mapa;
  if (!ehObjeto(bruto)) return mapa;
  for (const [origem, valor] of Object.entries(bruto)) {
    if (ehObjeto(valor) && typeof valor.desde === 'string') mapa[origem] = { desde: valor.desde };
  }
  return mapa;
}

export async function permitido(armazenamento: Armazenamento, origem: string): Promise<boolean> {
  const mapa = await lerMapa(armazenamento);
  return Object.hasOwn(mapa, origem);
}

export async function permitir(armazenamento: Armazenamento, origem: string, agora: Date = new Date()): Promise<void> {
  const mapa = await lerMapa(armazenamento);
  mapa[origem] = { desde: agora.toISOString() };
  await armazenamento.gravar(CHAVE_DAS_PERMISSOES, mapa);
}

export async function revogar(armazenamento: Armazenamento, origem: string): Promise<void> {
  const mapa = await lerMapa(armazenamento);
  if (!Object.hasOwn(mapa, origem)) return;
  delete mapa[origem];
  await armazenamento.gravar(CHAVE_DAS_PERMISSOES, mapa);
}

export async function listarPermissoes(armazenamento: Armazenamento): Promise<Permissao[]> {
  const mapa = await lerMapa(armazenamento);
  return Object.entries(mapa)
    .map(([origem, { desde }]) => ({ origem, desde }))
    .sort((a, b) => a.origem.localeCompare(b.origem));
}

/** O `storage.local` da extensão, no formato que este módulo usa. */
export function armazenamentoLocal(api: typeof chrome): Armazenamento {
  return {
    async ler(chave) {
      const r = await api.storage.local.get(chave);
      return r[chave];
    },
    async gravar(chave, valor) {
      await api.storage.local.set({ [chave]: valor });
    },
  };
}
