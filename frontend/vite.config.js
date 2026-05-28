import { defineConfig } from 'vite';

export default defineConfig({
  base: './',
  build: {
    rollupOptions: {
      external: [
        '/wails/runtime.js',
        '/wails/transport.js'
      ]
    }
  }
});
