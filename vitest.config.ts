import { defineConfig } from 'vitest/config';

export default defineConfig({
    test: {
        include: ['ts/**/*.test.ts'],
        environment: 'happy-dom',
    },
});
