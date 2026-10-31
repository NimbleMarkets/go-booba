import { copyFileSync, mkdirSync, readdirSync, rmSync, writeFileSync } from 'node:fs';

const source = new URL('../third_party/ntwebterm/dist/', import.meta.url);
const destination = new URL('../serve/static/ntwebterm/', import.meta.url);
const runtimeFiles = readdirSync(source, { withFileTypes: true })
    .filter(entry => entry.isFile() && /\.(js|wasm)$/.test(entry.name))
    .map(entry => entry.name);

for (const required of ['ntwebterm.js', 'ghostty-vt.wasm']) {
    if (!runtimeFiles.includes(required)) {
        throw new Error(`Missing prebuilt ntwebterm asset: ${required}`);
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

// Compatibility for booba 0.11; remove in 0.12.
const legacy = new URL('../serve/static/ghostty-web/', import.meta.url);
mkdirSync(legacy, { recursive: true });
for (const entry of readdirSync(legacy, { withFileTypes: true })) {
    if (entry.isFile()) rmSync(new URL(entry.name, legacy));
}
writeFileSync(new URL('ghostty-web.js', legacy), "export * from '../ntwebterm/ntwebterm.js';\n");
copyFileSync(new URL('ghostty-vt.wasm', source), new URL('ghostty-vt.wasm', legacy));
