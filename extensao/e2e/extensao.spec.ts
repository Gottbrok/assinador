import { chromium, expect, test, type BrowserContext, type Page } from '@playwright/test';

/**
 * A extensão de verdade, no Chromium, com o programa de desenvolvimento e o SoftHSM fazendo o papel
 * do cartão: a página pede, a extensão pergunta a permissão, lista, mostra a janela de confirmação,
 * e a assinatura que volta confere com o certificado no "servidor" (o `host-teste servir`).
 *
 * Tudo vem do orquestrador em Go (ver `playwright.config.ts`).
 */

const URL_DA_PAGINA = process.env.ASSINADOR_E2E_URL ?? '';
const DADOS = process.env.ASSINADOR_E2E_DADOS ?? '';
const EXTENSAO = process.env.ASSINADOR_E2E_EXTENSAO ?? '';
const PIN = process.env.ASSINADOR_E2E_PIN ?? '';
const REF = process.env.ASSINADOR_E2E_REF ?? '';
const ID_DEV = 'jmogljhnfdnhhoapijclifkpjfbhhppf';

test.skip(!URL_DA_PAGINA || !DADOS || !EXTENSAO, 'rode pelo orquestrador: npm run ponta-a-ponta');
// Um roteiro: cada passo depende do anterior (a permissão, o certificado listado).
test.describe.configure({ mode: 'serial' });

type Resposta = { ok: true; dados: Record<string, unknown> } | { ok: false; erro: { codigo: string; detalhe?: unknown } } | null;
type Certificado = { ref: string; der: string; exigePin: boolean };

let contexto: BrowserContext;
let pagina: Page;

test.beforeAll(async () => {
  contexto = await chromium.launchPersistentContext(DADOS, {
    channel: 'chromium',
    headless: process.env.E2E_HEADED !== '1',
    locale: 'pt-BR',
    args: [`--disable-extensions-except=${EXTENSAO}`, `--load-extension=${EXTENSAO}`],
  });
  const [sw] = contexto.serviceWorkers();
  const fundo = sw ?? (await contexto.waitForEvent('serviceworker'));
  expect(new URL(fundo.url()).host, 'o ID da extensão de desenvolvimento vem da chave pública do manifesto').toBe(ID_DEV);
  pagina = await contexto.newPage();
  await pagina.goto(URL_DA_PAGINA);
});

test.afterAll(async () => {
  await contexto?.close();
});

/** Os dados de uma resposta de sucesso (o teste já conferiu o `ok`). */
function dadosDe<T>(r: Resposta): T {
  if (!r?.ok) throw new Error(`resposta sem sucesso: ${JSON.stringify(r)}`);
  return r.dados as T;
}

const pedir = (op: string, dados?: unknown): Promise<Resposta> =>
  pagina.evaluate(([o, d]) => (window as unknown as { assinador: { pedir(op: string, dados?: unknown): Promise<unknown> } }).assinador.pedir(o as string, d), [op, dados] as const) as Promise<Resposta>;

/** Dispara `acao` e devolve a janela da extensão que ela abriu. */
async function janelaDe(pagina: string, acao: () => Promise<unknown>): Promise<{ janela: Page; resultado: Promise<unknown> }> {
  const aberta = contexto.waitForEvent('page', { predicate: (p) => p.url().startsWith(`chrome-extension://${ID_DEV}/${pagina}`) });
  const resultado = acao();
  const janela = await aberta;
  await janela.waitForLoadState('domcontentloaded');
  return { janela, resultado };
}

async function preparar(certificado: Certificado, documento: string) {
  return pagina.evaluate(
    async ([ref, doc]) => {
      const a = (window as unknown as { assinador: { resumo(t: string): Promise<string>; bilhete(r: string, d: string, doc: string): Promise<string> } }).assinador;
      const digest = await a.resumo(doc as string);
      return { digest, bilhete: await a.bilhete(ref as string, digest, doc as string) };
    },
    [certificado.ref, documento] as const,
  );
}

test('ola responde as duas versões, sem permissão e sem janela', async () => {
  const r = await pedir('ola');
  expect(r).toEqual({ ok: true, dados: { extensao: { versao: '1.0.0' }, nativo: { versao: '1.0.0', protocolo: 1, plataforma: expect.stringMatching(/^linux-/) } } });
  expect(contexto.pages().filter((p) => p.url().startsWith('chrome-extension://'))).toHaveLength(0);
});

let certificado: Certificado;

test('listar pede a permissão do endereço: negar recusa, permitir lista o certificado do token', async () => {
  const host = new URL(URL_DA_PAGINA).host;
  const negada = await janelaDe('permitir.html', () => pedir('listar'));
  await expect(negada.janela.locator('#host')).toHaveText(host);
  await negada.janela.locator('#negar').click();
  expect(await negada.resultado).toEqual({ ok: false, erro: { codigo: 'permissao-negada' } });

  const permitida = await janelaDe('permitir.html', () => pedir('listar'));
  await permitida.janela.locator('#permitir').click();
  const r = (await permitida.resultado) as Resposta;
  expect(r?.ok).toBe(true);
  const lista = dadosDe<{ certificados: Certificado[] }>(r).certificados;
  const achado = lista.find((c) => c.ref === REF);
  expect(achado, `o titular do token (${REF}) está na lista`).toBeDefined();
  certificado = achado as Certificado;

  // A segunda vez não pergunta.
  expect((await pedir('listar'))?.ok).toBe(true);
});

test('assinar mostra na janela o que o bilhete e o navegador dizem; cancelar é `cancelado`', async () => {
  const { digest, bilhete } = await preparar(certificado, 'Contrato que vai ser cancelado');
  const { janela, resultado } = await janelaDe('confirmar.html', () => pedir('assinar', { ref: certificado.ref, digest, bilhete }));
  await expect(janela.locator('#host')).toHaveText(new URL(URL_DA_PAGINA).host);
  await expect(janela.locator('#documento')).toHaveText('Contrato que vai ser cancelado');
  await expect(janela.locator('#organizacao')).toHaveText('Assinador (teste local)');
  await expect(janela.locator('#finalidade')).toHaveText('Assinar documento');
  // O titular sem o CPF (o CN ICP-Brasil é NOME:CPF).
  await expect(janela.locator('#titular')).toHaveText('TITULAR DE TESTE');
  await expect(janela.locator('body')).not.toContainText('12345678901');
  await expect(janela.locator('#pin')).toBeVisible();
  await janela.locator('#cancelar').click();
  expect(await resultado).toMatchObject({ ok: false, erro: { codigo: 'cancelado' } });
});

test('assinar com o PIN devolve a assinatura, e ela confere com o certificado', async () => {
  const documento = 'Contrato de teste da ponta a ponta';
  const { digest, bilhete } = await preparar(certificado, documento);
  const { janela, resultado } = await janelaDe('confirmar.html', () => pedir('assinar', { ref: certificado.ref, digest, bilhete }));
  await janela.locator('#pin').fill(PIN);
  await janela.locator('#assinar').click();
  const r = (await resultado) as Resposta;
  expect(r?.ok, JSON.stringify(r)).toBe(true);
  const assinatura = dadosDe<{ assinatura: string }>(r).assinatura;
  const conferencia = await pagina.evaluate(
    ([der, d, a]) => (window as unknown as { assinador: { conferir(der: string, d: string, a: string): Promise<unknown> } }).assinador.conferir(der as string, d as string, a as string),
    [certificado.der, digest, assinatura] as const,
  );
  expect(conferencia).toEqual({ confere: true });
});

test('bilhete para outro resumo é recusado pelo PROGRAMA, antes de qualquer janela', async () => {
  const { bilhete } = await preparar(certificado, 'Outro documento');
  const outroResumo = 'f'.repeat(64);
  const r = await pedir('assinar', { ref: certificado.ref, digest: outroResumo, bilhete });
  expect(r).toMatchObject({ ok: false, erro: { codigo: 'digest-divergente' } });
  expect(contexto.pages().filter((p) => p.url().includes('confirmar.html'))).toHaveLength(0);
});

test('as opções listam o endereço, as versões e o diagnóstico sem CPF; remover volta a pedir permissão', async () => {
  const opcoes = await contexto.newPage();
  await opcoes.goto(`chrome-extension://${ID_DEV}/opcoes.html`);
  const origem = new URL(URL_DA_PAGINA).origin;
  await expect(opcoes.locator('#permissoes li')).toHaveCount(1);
  await expect(opcoes.locator('#permissoes .origem')).toHaveText(origem);
  await expect(opcoes.locator('#versao-programa')).toContainText('1.0.0');
  await opcoes.locator('#gerar-diagnostico').click();
  await expect(opcoes.locator('#diagnostico')).toBeVisible({ timeout: 30_000 });
  await expect(opcoes.locator('#diagnostico')).toContainText('Extensão: versão 1.0.0');
  await expect(opcoes.locator('#diagnostico')).not.toContainText('12345678901');
  await opcoes.locator('#permissoes button').click();
  await expect(opcoes.locator('#permissoes li')).toHaveCount(0);
  await expect(opcoes.locator('#permissoes-vazia')).toBeVisible();
  await opcoes.close();

  const denovo = await janelaDe('permitir.html', () => pedir('listar'));
  await denovo.janela.close();
  expect(await denovo.resultado).toEqual({ ok: false, erro: { codigo: 'permissao-negada' } });
});
