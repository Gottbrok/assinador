#!/usr/bin/env node
/**
 * Gera os ícones PROVISÓRIOS da extensão (`icones/16.png` a `icones/128.png`): um quadrado de cantos
 * arredondados escuro com um visto branco. Ficam no repositório, e o build só os copia (arquivo que
 * falta é erro, nunca recuo). A marca definitiva entra com o nome da extensão (decisão D2 do plano,
 * antes da publicação).
 *
 * O 128 segue a regra da Chrome Web Store: arte em 96 px com 16 px de margem transparente. O desenho
 * é por amostragem 4x4 por pixel, sem biblioteca, e sai igual byte a byte a cada execução.
 *
 * Uso: node scripts/gerar-icones-provisorios.mjs
 */
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { crc32, deflateSync } from 'node:zlib';

const aqui = dirname(fileURLToPath(import.meta.url));
const pasta = resolve(aqui, '..', 'icones');
const FUNDO = [0x1b, 0x1f, 0x24];
const TRACO = [0xff, 0xff, 0xff];
const AMOSTRAS = 4;

/** Distância do ponto ao segmento AB. */
function distancia(px, py, ax, ay, bx, by) {
  const dx = bx - ax;
  const dy = by - ay;
  const t = Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / (dx * dx + dy * dy)));
  return Math.hypot(px - (ax + t * dx), py - (ay + t * dy));
}

/** 0 fora, 1 no fundo, 2 no traço, para um ponto em coordenadas da arte (0 a 1). */
function regiao(x, y) {
  const r = 0.2;
  const cx = Math.min(Math.max(x, r), 1 - r);
  const cy = Math.min(Math.max(y, r), 1 - r);
  if (x < 0 || y < 0 || x > 1 || y > 1 || Math.hypot(x - cx, y - cy) > r) return 0;
  const meia = 0.055;
  if (distancia(x, y, 0.27, 0.52, 0.43, 0.68) <= meia || distancia(x, y, 0.43, 0.68, 0.74, 0.34) <= meia) return 2;
  return 1;
}

function desenhar(tamanho) {
  const margem = tamanho === 128 ? 16 : 0;
  const arte = tamanho - 2 * margem;
  const linhas = [];
  for (let y = 0; y < tamanho; y += 1) {
    const linha = Buffer.alloc(1 + tamanho * 4);
    for (let x = 0; x < tamanho; x += 1) {
      let fundo = 0;
      let traco = 0;
      for (let sy = 0; sy < AMOSTRAS; sy += 1) {
        for (let sx = 0; sx < AMOSTRAS; sx += 1) {
          const r = regiao((x - margem + (sx + 0.5) / AMOSTRAS) / arte, (y - margem + (sy + 0.5) / AMOSTRAS) / arte);
          if (r === 1) fundo += 1;
          if (r === 2) traco += 1;
        }
      }
      const cobertura = fundo + traco;
      const o = 1 + x * 4;
      if (cobertura === 0) continue;
      for (let c = 0; c < 3; c += 1) linha[o + c] = Math.round((FUNDO[c] * fundo + TRACO[c] * traco) / cobertura);
      linha[o + 3] = Math.round((255 * cobertura) / (AMOSTRAS * AMOSTRAS));
    }
    linhas.push(linha);
  }
  return png(tamanho, Buffer.concat(linhas));
}

function bloco(tipo, dados) {
  const t = Buffer.from(tipo, 'ascii');
  const tamanho = Buffer.alloc(4);
  tamanho.writeUInt32BE(dados.length);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(Buffer.concat([t, dados])));
  return Buffer.concat([tamanho, t, dados, crc]);
}

function png(tamanho, cru) {
  const cabecalho = Buffer.alloc(13);
  cabecalho.writeUInt32BE(tamanho, 0);
  cabecalho.writeUInt32BE(tamanho, 4);
  cabecalho[8] = 8; // bits por canal
  cabecalho[9] = 6; // RGBA
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    bloco('IHDR', cabecalho),
    bloco('IDAT', deflateSync(cru, { level: 9 })),
    bloco('IEND', Buffer.alloc(0)),
  ]);
}

mkdirSync(pasta, { recursive: true });
for (const tamanho of [16, 32, 48, 128]) {
  writeFileSync(resolve(pasta, `${tamanho}.png`), desenhar(tamanho));
}
console.log(`ícones provisórios em ${pasta}`);
