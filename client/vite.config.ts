import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig, loadEnv } from 'vite';

const src = path.resolve(path.dirname(fileURLToPath(import.meta.url)), 'src');

export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, process.cwd(), '');
    const liveApi = env.VITE_API_MODE === 'live';

    return {
        plugins: [react(), tailwindcss()],
        resolve: {
            alias: {
                app: path.join(src, 'app'),
                pages: path.join(src, 'pages'),
                features: path.join(src, 'features'),
                shared: path.join(src, 'shared'),
                '@': src,
            },
            dedupe: ['react', 'react-dom'],
        },
        server: {
            port: 3000,
            open: false,
            proxy: liveApi
                ? {
                      '/api': {
                          target: 'http://localhost:8080',
                          changeOrigin: true,
                      },
                  }
                : undefined,
        },
    };
});
