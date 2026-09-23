import tailwindcss from '@tailwindcss/vite';
import { defineConfig, type Plugin } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';

const apiTarget = process.env.LOINC_API_TARGET || 'http://localhost:8080';

// Bundle Swagger UI into dist/vendor/swagger-ui so /api/docs works without internet.
function swaggerUIAssets(): Plugin {
  const swaggerDir = path.dirname(createRequire(import.meta.url).resolve('swagger-ui-dist/package.json'));
  return {
    name: 'swagger-ui-assets',
    apply: 'build',
    generateBundle() {
      for (const file of ['swagger-ui.css', 'swagger-ui-bundle.js', 'LICENSE', 'NOTICE']) {
        this.emitFile({ type: 'asset', fileName: `vendor/swagger-ui/${file}`, source: readFileSync(path.join(swaggerDir, file)) });
      }
    },
  };
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [tailwindcss(), svelte(), swaggerUIAssets()],
  server: {
    proxy: {
      '/api': apiTarget,
      '/openapi.json': apiTarget,
      '/fhir': apiTarget,
      '/searchapi': apiTarget,
      '/docs': apiTarget,
      '/vendor': apiTarget,
    },
  },
  resolve: {
    alias: {
      $lib: path.resolve('./src/lib'),
    },
  },
});
