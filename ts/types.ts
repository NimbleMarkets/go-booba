/**
 * Booba type definitions
 *
 * Re-exports of ghostty-web types that are part of booba's public API,
 * plus booba-specific types.
 */

/** Terminal theme colors */
export interface BoobaTheme {
    foreground?: string;
    background?: string;
    cursor?: string;
    cursorAccent?: string;
    selectionBackground?: string;
    selectionForeground?: string;
    black?: string;
    red?: string;
    green?: string;
    yellow?: string;
    blue?: string;
    magenta?: string;
    cyan?: string;
    white?: string;
    brightBlack?: string;
    brightRed?: string;
    brightGreen?: string;
    brightYellow?: string;
    brightBlue?: string;
    brightMagenta?: string;
    brightCyan?: string;
    brightWhite?: string;
}

/** Buffer range for selection positions */
export interface BoobaBufferRange {
    start: { x: number; y: number };
    end: { x: number; y: number };
}

/** Keyboard event from the terminal */
export interface BoobaKeyEvent {
    key: string;
    domEvent: KeyboardEvent;
}

/** Render event range */
export interface BoobaRenderEvent {
    start: number;
    end: number;
}

/** Link provider interface for registering custom link detection */
export interface BoobaLinkProvider {
    provideLinks(y: number, callback: (links: BoobaLink[] | undefined) => void): void;
    dispose?(): void;
}

/** A detected link in the terminal */
export interface BoobaLink {
    text: string;
    range: BoobaBufferRange;
    activate(event: MouseEvent): void;
    hover?(isHovered: boolean): void;
    dispose?(): void;
}
