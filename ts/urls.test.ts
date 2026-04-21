import { describe, it, expect } from 'vitest';
import { resolveBoobaURLs } from './urls.js';

describe('resolveBoobaURLs', () => {
    it('builds ws/wt/cert-hash URLs at the site root for http://', () => {
        const urls = resolveBoobaURLs('http://localhost:8080/');
        expect(urls.wsUrl).toBe('ws://localhost:8080/ws');
        expect(urls.wtUrl).toBe('https://localhost:8080/wt');
        expect(urls.certHashUrl).toBe('http://localhost:8080/cert-hash');
    });

    it('upgrades ws → wss under https', () => {
        const urls = resolveBoobaURLs('https://host.example/');
        expect(urls.wsUrl).toBe('wss://host.example/ws');
    });

    it('preserves a path prefix from the document base', () => {
        const urls = resolveBoobaURLs('https://host.example/terminal/');
        expect(urls.wsUrl).toBe('wss://host.example/terminal/ws');
        expect(urls.wtUrl).toBe('https://host.example/terminal/wt');
        expect(urls.certHashUrl).toBe('https://host.example/terminal/cert-hash');
    });

    it('treats a non-slash-terminated baseURI as the containing directory', () => {
        // baseURI of a page loaded at /terminal/index.html should resolve
        // endpoints relative to /terminal/, not /terminal/index.html/.
        const urls = resolveBoobaURLs('https://host.example/terminal/index.html');
        expect(urls.wsUrl).toBe('wss://host.example/terminal/ws');
    });

    it('handles deeply-nested prefixes', () => {
        const urls = resolveBoobaURLs('https://host.example/apps/booba/session/');
        expect(urls.wsUrl).toBe('wss://host.example/apps/booba/session/ws');
        expect(urls.wtUrl).toBe('https://host.example/apps/booba/session/wt');
        expect(urls.certHashUrl).toBe('https://host.example/apps/booba/session/cert-hash');
    });

    it('keeps custom ports on all three URLs', () => {
        const urls = resolveBoobaURLs('http://127.0.0.1:9999/terminal/');
        expect(urls.wsUrl).toBe('ws://127.0.0.1:9999/terminal/ws');
        expect(urls.wtUrl).toBe('https://127.0.0.1:9999/terminal/wt');
    });
});
