/** Helpers shared by booba's host-styled DOM (action bar, status bar). */

/** Inject one `<style>` per attribute per document; later calls are no-ops. */
export function ensureStyles(doc: Document, attribute: string, css: string): void {
    if (doc.head.querySelector(`style[${attribute}]`)) return;
    const style = doc.createElement('style');
    style.setAttribute(attribute, '');
    style.textContent = css;
    doc.head.append(style);
}

/**
 * Strip invisible direction overrides and formatting characters from program
 * text, so a label or message cannot reorder or hide host UI around it.
 */
export function displayText(value: string): string {
    return value.replace(/[\u200b-\u200f\u202a-\u202e\u2060-\u206f\ufeff]/gu, '');
}
