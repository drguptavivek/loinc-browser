import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'node:path';

const apiTarget = process.env.LOINC_API_TARGET || 'http://localhost:8080';

// https://vite.dev/config/
export default defineConfig({
  plugins: [tailwindcss(), svelte()],
  server: {
    proxy: {
      '/api': apiTarget,
      '/openapi.json': apiTarget,
    },
  },
  resolve: {
    alias: {
      $lib: path.resolve('./src/lib'),
    },
  },
});
