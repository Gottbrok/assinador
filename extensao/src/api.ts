/**
 * A API de extensão com promessas, a mesma nos dois navegadores: `browser` no Firefox (onde o
 * `chrome` responde por callback), `chrome` no Chrome e no Edge (Manifest V3, que devolve promessa).
 * `null` fora de uma extensão (os testes injetam o que precisam).
 */
export function navegador(): typeof chrome | null {
  const g = globalThis as { browser?: typeof chrome; chrome?: typeof chrome };
  const api = g.browser ?? g.chrome;
  return api?.runtime?.id ? api : null;
}
