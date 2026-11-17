import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
    plugins: [tailwindcss(), sveltekit()],
    server: {
        host: '127.0.0.1',
        port: 5190,
        strictPort: true,
        // The interface calls the wyo server's own API at the same address it was loaded from; in development the wyo
        // server listens on its own port.
        proxy: {
            '/api': 'http://127.0.0.1:8790'
        },
        // `loadMessages` and the language files are plain files outside what SvelteKit opens.
        fs: { allow: ['messages'] }
    }
});
