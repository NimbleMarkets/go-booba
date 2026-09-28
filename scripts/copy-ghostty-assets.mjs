import { copyFileSync, mkdirSync, readdirSync, rmSync } from 'node:fs';

const source = new URL('../third_party/ghostty-web/dist/', import.meta.url);
const destination = new URL('../serve/static/ghostty-web/', import.meta.url);
const runtimeFiles = readdirSync(source, { withFileTypes: true })
    .filter(entry => entry.isFile() && /\.(js|wasm)$/.test(entry.name))
    .map(entry => entry.name);

for (const required of ['ghostty-web.js', 'ghostty-vt.wasm']) {
    if (!runtimeFiles.includes(required)) {
        throw new Error(`Missing prebuilt ghostty-web asset: ${required}`);
    }
}

mkdirSync(destination, { recursive: true });
for (const name of runtimeFiles) {
    copyFileSync(new URL(name, source), new URL(name, destination));
}
// This directory contains generated copies only. Prune old Vite chunks and
// artifacts from earlier builds that copied the entire upstream distribution.
for (const entry of readdirSync(destination, { withFileTypes: true })) {
    if (entry.isFile() && !runtimeFiles.includes(entry.name)) {
        rmSync(new URL(entry.name, destination));
    }
}
