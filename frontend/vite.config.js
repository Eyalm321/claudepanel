import { defineConfig } from 'vite';
import { resolve } from 'path';
import { fileURLToPath } from 'url';

const __dirname = fileURLToPath(new URL('.', import.meta.url));

export default defineConfig({
  base: './',
  build: {
    rollupOptions: {
      // Two HTML entries: the bar (index.html) and the settings popup
      // (settings.html), both built into dist/ and served by the Go asset server.
      input: {
        main: resolve(__dirname, 'index.html'),
        settings: resolve(__dirname, 'settings.html'),
      },
      external: [
        '/wails/runtime.js',
        '/wails/transport.js'
      ]
    }
  }
});
