import type { BoobaTheme } from './types.js';

/**
 * Mirror the colors the canvas is painted with onto the container as CSS
 * custom properties, so host chrome around the terminal can match it with
 * `var(--booba-background)` and `var(--booba-foreground)` instead of
 * repeating hex values. Only the two base colors are exposed.
 */
export function applyThemeProperties(element: HTMLElement, theme: BoobaTheme | undefined): void {
    if (!theme) return;
    if (theme.background) element.style.setProperty('--booba-background', theme.background);
    if (theme.foreground) element.style.setProperty('--booba-foreground', theme.foreground);
}
