import { loadEnv } from 'vite';
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig(({ mode }) => {
  const fileEnv = loadEnv(mode, process.cwd(), '');
  const env = { ...fileEnv, ...process.env };

  return {
    plugins: [react()],
    define: {
      'import.meta.env.REACT_APP_WS_URL': JSON.stringify(env.REACT_APP_WS_URL || ''),
      'import.meta.env.REACT_APP_WS_PORT': JSON.stringify(env.REACT_APP_WS_PORT || ''),
      'import.meta.env.REACT_APP_API_URL': JSON.stringify(env.REACT_APP_API_URL || ''),
      'import.meta.env.REACT_APP_API_PORT': JSON.stringify(env.REACT_APP_API_PORT || ''),
    },
    server: {
      host: '0.0.0.0',
      port: 3000,
    },
    preview: {
      host: '0.0.0.0',
      port: 3000,
    },
    build: {
      outDir: 'build',
    },
    test: {
      clearMocks: true,
      css: true,
      environment: 'jsdom',
      exclude: ['**/node_modules/**', '**/dist/**', '**/e2e/**'],
      environmentOptions: {
        jsdom: {
          url: 'http://localhost',
        },
      },
      globals: true,
      setupFiles: './src/setupTests.js',
    },
  };
});
