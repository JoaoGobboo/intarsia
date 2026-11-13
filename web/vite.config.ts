import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  build: {
    // The Go binary embeds this directory (see cmd/intarsia/assets.go).
    outDir: '../cmd/intarsia/dist',
    emptyOutDir: true,
  },
});
