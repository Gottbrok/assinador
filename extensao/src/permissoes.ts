/**
 * Permissão por endereço (§3.3 e §3.6 do plano).
 *
 * `listar`, `assinar` e `diagnostico` só atendem uma origem que a pessoa autorizou nesta extensão.
 * Sem isso, qualquer script que rodasse numa página nossa leria em silêncio o nome e o CPF de todos
 * os certificados do computador (o CN ICP-Brasil é `NOME:CPF`). `ola` não precisa: diz só versões.
 *
 * A decisão fica em `storage.local`, NUNCA no `storage.sync` (permissão num computador não vale para
 * outro), por ORIGEM (esquema, host e porta): `https://exemplo.confidata.app` autorizado não autoriza
 * `https://outra.confidata.app`. A página de opções lista e revoga uma a uma.
 *
 * Uma CHAVE por origem (`permissao:<origem>`), e não um mapa numa chave só: o fundo grava e a página
 * de opções apaga, em contextos diferentes, e ler o mapa, mudar e regravar deixava uma escrita
 * desfazer a outra (uma revogação voltava, uma permissão sumia). Gravar e apagar uma chave são
 * atômicos no `storage`.
 */

export const PREFIXO_DA_PERMISSAO = 'permissao:';

export interface Permissao {
  origem: string;
  /** Quando a pessoa permitiu (ISO 8601). */
  desde: string;
}

export interface Armazenamento {
  ler(chave: string): Promise<unknown>;
  lerTudo(): Promise<Record<string, unknown>>;
  gravar(chave: string, valor: unknown): Promise<void>;
  apagar(chave: string): Promise<void>;
}

const chaveDa = (origem: string) => `${PREFIXO_DA_PERMISSAO}${origem}`;

function desdeDe(valor: unknown): string | null {
  if (typeof valor !== 'object' || valor === null || Array.isArray(valor)) return null;
  const desde = (valor as { desde?: unknown }).desde;
  return typeof desde === 'string' ? desde : null;
}

/** Só a entrada com forma é permissão; o resto é ignorado, nunca vira "permitido". */
export async function permitido(armazenamento: Armazenamento, origem: string): Promise<boolean> {
  return desdeDe(await armazenamento.ler(chaveDa(origem))) !== null;
}

export async function permitir(armazenamento: Armazenamento, origem: string, agora: Date = new Date()): Promise<void> {
  await armazenamento.gravar(chaveDa(origem), { desde: agora.toISOString() });
}

export async function revogar(armazenamento: Armazenamento, origem: string): Promise<void> {
  await armazenamento.apagar(chaveDa(origem));
}

export async function listarPermissoes(armazenamento: Armazenamento): Promise<Permissao[]> {
  const tudo = await armazenamento.lerTudo();
  const lista: Permissao[] = [];
  for (const [chave, valor] of Object.entries(tudo)) {
    if (!chave.startsWith(PREFIXO_DA_PERMISSAO)) continue;
    const desde = desdeDe(valor);
    if (desde !== null) lista.push({ origem: chave.slice(PREFIXO_DA_PERMISSAO.length), desde });
  }
  return lista.sort((a, b) => a.origem.localeCompare(b.origem));
}

/** A chave de `storage.onChanged` é de permissão? (a página de opções redesenha a lista). */
export function ehChaveDePermissao(chave: string): boolean {
  return chave.startsWith(PREFIXO_DA_PERMISSAO);
}

/** O `storage.local` da extensão, no formato que este módulo usa. */
export function armazenamentoLocal(api: typeof chrome): Armazenamento {
  return {
    async ler(chave) {
      const r = await api.storage.local.get(chave);
      return Object.hasOwn(r, chave) ? r[chave] : undefined;
    },
    async lerTudo() {
      return api.storage.local.get(null);
    },
    async gravar(chave, valor) {
      await api.storage.local.set({ [chave]: valor });
    },
    async apagar(chave) {
      await api.storage.local.remove(chave);
    },
  };
}
