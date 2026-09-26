#!/usr/bin/env node
/**
 * Build da extensão, uma fonte para os dois alvos (§3.6 do plano).
 *
 *   node build.mjs            dist/chrome e dist/firefox (os builds da loja, sem localhost)
 *   node build.mjs --dev      dist/chrome-dev e dist/firefox-dev (localhost e a chave pública de dev)
 *   node build.mjs --zip      os builds da loja e os pacotes em pacotes/, reproduzíveis
 *
 * Cada script (`conteudo`, `fundo`, `confirmar`, `permitir`, `opcoes`) sai num arquivo só, IIFE,
 * SEM minificar: é superfície de segurança, e quem revisa (a loja, quem audita o código) lê o que
 * roda. O script de conteúdo não pode ser módulo, e um arquivo por script evita pedaço compartilhado
 * que um navegador carregaria e o outro não.
 *
 * O pacote é REPRODUZÍVEL (dois builds da mesma árvore, mesmo SHA-256; `npm run reproduzivel`
 * confere): `mode` fixo em `production` (o `NODE_ENV` de quem builda não entra no bundle), mtime
 * fixo (SOURCE_DATE_EPOCH, ou a data do commit), entradas em ordem, permissões 0644, sem atributos
 * extras (`zip -X`), e o horário do zip em UTC (o formato DOS guarda hora local).
 *
 * O pacote do Chrome sai SEM `key`: a Chrome Web Store recusa o campo (medido na extensão do
 * Datashield em 2026-09-12), e o ID do item vem da loja.
 */
import { execFileSync } from 'node:child_process';
import { chmodSync, cpSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, utimesSync, writeFileSync } from 'node:fs';
import { dirname, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { build } from 'vite';

import { manifesto } from './manifest.base.ts';

const aqui = dirname(fileURLToPath(import.meta.url));
const argumentos = new Set(process.argv.slice(2));
for (const a of argumentos) if (!['--dev', '--zip'].includes(a)) throw new Error(`argumento desconhecido: ${a}`);
const dev = argumentos.has('--dev');
const zip = argumentos.has('--zip');
if (dev && zip) throw new Error('--dev e --zip não andam juntos: o pacote da loja não leva localhost nem a chave de desenvolvimento');

const MODO = 'production';
process.env.NODE_ENV = MODO;

const ENTRADAS = ['conteudo', 'fundo', 'confirmar', 'permitir', 'opcoes'];
const PAGINAS = ['confirmar.html', 'permitir.html', 'opcoes.html', 'paginas.css'];
const ICONES = ['16.png', '32.png', '48.png', '128.png'];
const versao = JSON.parse(readFileSync(resolve(aqui, 'package.json'), 'utf8')).version;
const chaveDev = JSON.parse(readFileSync(resolve(aqui, '..', 'protocolo', 'extensao-dev.json'), 'utf8')).chave;

for (const alvo of ['chrome', 'firefox']) {
  const saida = resolve(aqui, 'dist', dev ? `${alvo}-dev` : alvo);
  rmSync(saida, { recursive: true, force: true });
  mkdirSync(saida, { recursive: true });
  for (const entrada of ENTRADAS) {
    await build({
      configFile: false,
      mode: MODO,
      logLevel: 'warn',
      publicDir: false,
      define: { __ASSINADOR_DEV__: JSON.stringify(dev) },
      build: {
        outDir: saida,
        emptyOutDir: false,
        minify: false,
        sourcemap: false,
        target: 'es2022',
        reportCompressedSize: false,
        lib: { entry: resolve(aqui, 'src', `${entrada}.ts`), formats: ['iife'], name: `assinador_${entrada}`, fileName: () => `${entrada}.js` },
      },
    });
  }
  for (const p of PAGINAS) cpSync(resolve(aqui, 'paginas', p), resolve(saida, p));
  cpSync(resolve(aqui, '_locales'), resolve(saida, '_locales'), { recursive: true });
  mkdirSync(resolve(saida, 'icones'));
  for (const i of ICONES) {
    const origem = resolve(aqui, 'icones', i);
    if (!existsSync(origem)) throw new Error(`ícone ausente: ${origem} (gere com scripts/gerar-icones-provisorios.mjs)`);
    cpSync(origem, resolve(saida, 'icones', i));
  }
  const m = manifesto({ alvo, versao, dev, ...(alvo === 'chrome' && dev ? { chaveDev } : {}) });
  writeFileSync(resolve(saida, 'manifest.json'), `${JSON.stringify(m, null, 2)}\n`);
  conferirSemRede(saida);
  console.log(`${alvo}${dev ? ' (desenvolvimento)' : ''}: ${relative(aqui, saida)}`);
  if (zip) empacotar(saida, resolve(aqui, 'pacotes', `assinador-extensao-${alvo}.zip`));
}

/**
 * Catraca do bundle: a extensão não acessa a rede (regra 3 do CLAUDE.md). Se um dia uma dependência
 * trouxer `fetch`, `XMLHttpRequest`, `WebSocket` ou `EventSource` para dentro do pacote, o build para.
 */
function conferirSemRede(saida) {
  for (const arquivo of listarArquivos(saida).filter((a) => a.endsWith('.js'))) {
    const codigo = readFileSync(arquivo, 'utf8');
    const achado = /\b(fetch|XMLHttpRequest|WebSocket|EventSource|sendBeacon)\s*\(/.exec(codigo) ?? /\bnew\s+(XMLHttpRequest|WebSocket|EventSource)\b/.exec(codigo);
    if (achado) throw new Error(`${relative(aqui, arquivo)} usa ${achado[1]}: a extensão não acessa a rede`);
  }
}

function empacotar(pasta, destino) {
  mkdirSync(dirname(destino), { recursive: true });
  rmSync(destino, { force: true });
  const epoch = Number(process.env.SOURCE_DATE_EPOCH || dataDoCommit() || Date.UTC(2026, 0, 1) / 1000);
  const carimbo = new Date(epoch * 1000);
  const arquivos = listarArquivos(pasta);
  for (const a of arquivos) {
    chmodSync(a, 0o644);
    utimesSync(a, carimbo, carimbo);
  }
  const entradas = arquivos.map((a) => relative(pasta, a)).sort();
  execFileSync('zip', ['-X', '-D', '-q', destino, ...entradas], { cwd: pasta, stdio: 'inherit', env: { ...process.env, TZ: 'UTC' } });
  console.log(`  pacote ${relative(aqui, destino)} (mtime ${carimbo.toISOString()})`);
}

function dataDoCommit() {
  try {
    return execFileSync('git', ['log', '-1', '--format=%ct'], { cwd: aqui, encoding: 'utf8' }).trim();
  } catch {
    return '';
  }
}

function listarArquivos(raiz) {
  const saida = [];
  for (const nome of readdirSync(raiz, { withFileTypes: true })) {
    const caminho = resolve(raiz, nome.name);
    if (nome.isDirectory()) saida.push(...listarArquivos(caminho));
    else saida.push(caminho);
  }
  return saida;
}
