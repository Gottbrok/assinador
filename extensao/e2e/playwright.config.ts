import { defineConfig } from '@playwright/test';

/**
 * O teste ponta a ponta da extensão. Quem o roda é o orquestrador em Go
 * (`nativo/cmd/assinador/extensao_test.go`, tag `extensao`, pelo `npm run ponta-a-ponta`): ele monta
 * o token SoftHSM, o programa de desenvolvimento, o manifesto do host no diretório de dados do
 * navegador e a página de teste, e passa tudo por variáveis de ambiente. Rodado sozinho, o teste pula.
 */
export default defineConfig({
  testDir: '.',
  testMatch: /.*\.spec\.ts$/,
  timeout: 180_000,
  workers: 1,
  fullyParallel: false,
  retries: 0,
  reporter: [['list']],
  outputDir: '../test-results',
});
