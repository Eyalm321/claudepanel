import { defineConfig } from 'vite';
import { resolve } from 'path';
import { fileURLToPath } from 'url';

const __dirname = fileURLToPath(new URL('.', import.meta.url));

export default defineConfig({
  base: './',
  build: {
    rollupOptions: {
      // Three HTML entries: the bar (index.html), the settings popup
      // (settings.html) and the brand dropdown (menu.html), all built into dist/
      // and served by the Go asset server.
      input: {
        main: resolve(__dirname, 'index.html'),
        settings: resolve(__dirname, 'settings.html'),
        menu: resolve(__dirname, 'menu.html'),
      },
      external: [
        '/wails/runtime.js',
        '/wails/transport.js'
      ]
    }
  }
});
