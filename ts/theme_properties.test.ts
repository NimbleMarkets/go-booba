import { describe, expect, it } from 'vitest';
import { applyThemeProperties } from './theme_properties.js';

describe('applyThemeProperties', () => {
    it('exposes background and foreground as CSS custom properties on the element', () => {
        const el = document.createElement('div');
        applyThemeProperties(el, { background: '#10151d', foreground: '#e4edf7' });
        expect(el.style.getPropertyValue('--booba-background')).toBe('#10151d');
        expect(el.style.getPropertyValue('--booba-foreground')).toBe('#e4edf7');
    });

    it('omits a property whose theme value is absent and ignores other theme keys', () => {
        const el = document.createElement('div');
        applyThemeProperties(el, { foreground: '#fff', cursor: '#f00' });
        expect(el.style.getPropertyValue('--booba-background')).toBe('');
        expect(el.style.getPropertyValue('--booba-foreground')).toBe('#fff');
        expect(el.style.getPropertyValue('--booba-cursor')).toBe('');
        applyThemeProperties(el, undefined);
        expect(el.style.getPropertyValue('--booba-foreground')).toBe('#fff');
    });
});
