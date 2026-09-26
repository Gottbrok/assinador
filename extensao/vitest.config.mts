import { defineConfig } from 'vitest/config';

export default defineConfig({
  define: { __ASSINADOR_DEV__: 'false' },
  test: {
    include: ['__tests__/**/*.test.ts'],
    environment: 'node',
    // Os testes são pequenos e sem navegador: um processo basta, e não disputa a máquina.
    pool: 'threads',
    maxWorkers: 2,
  },
});
