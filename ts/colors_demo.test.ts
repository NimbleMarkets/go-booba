import { expect, it, vi } from 'vitest';
import { installColorsDemo } from '../web/_assets/colors.js';

it('Show all retains the earlier color samples when adding text styles', () => {
    document.body.innerHTML = '<div id="terminal"><textarea></textarea></div><button data-color-sample="all" disabled>Show all</button>';
    let screen = '';
    const terminal = {
        container: document.getElementById('terminal')!,
        term: { textarea: document.querySelector('textarea')! },
        write: (text: string) => { screen += text; },
        clear: vi.fn(() => { screen = ''; }),
        dispose: vi.fn(),
    };
    installColorsDemo(terminal);
    terminal.clear.mockClear();
    document.querySelector<HTMLButtonElement>('button')!.click();
    expect(terminal.clear).toHaveBeenCalledTimes(1);
    for (const sample of ['Standard 16 ANSI Colors', '256-Color Palette', 'RGB True Color', 'Text Styles']) {
        expect(screen).toContain(sample);
    }
});
