#!/usr/bin/env node
/**
 * Prova de build reproduzível: monta os pacotes duas vezes a partir da mesma árvore, com ambientes
 * DIFERENTES (fuso e `NODE_ENV`), e compara o SHA-256 de cada um. Divergência é erro (código 1): quem
 * audita o código precisa conseguir refazer o pacote noutra máquina e conferir, byte a byte, que o
 * que está na loja é o que foi auditado.
 *
 * Uso: node scripts/conferir-reproducao.mjs
 */
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const raiz = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const PACOTES = ['assinador-extensao-chrome.zip', 'assinador-extensao-firefox.zip'];

function montar(ambiente) {
  execFileSync(process.execPath, ['build.mjs', '--zip'], { cwd: raiz, stdio: ['ignore', 'ignore', 'inherit'], env: { ...process.env, ...ambiente } });
  return Object.fromEntries(PACOTES.map((p) => [p, createHash('sha256').update(readFileSync(resolve(raiz, 'pacotes', p))).digest('hex')]));
}

const primeira = montar({ TZ: 'UTC', NODE_ENV: 'production' });
const segunda = montar({ TZ: 'Pacific/Kiritimati', NODE_ENV: 'development' });
let divergiu = false;
for (const p of PACOTES) {
  const igual = primeira[p] === segunda[p];
  divergiu ||= !igual;
  console.log(`${igual ? 'igual' : 'DIVERGE'}  ${p}  ${primeira[p]}${igual ? '' : ` != ${segunda[p]}`}`);
}
process.exit(divergiu ? 1 : 0);
