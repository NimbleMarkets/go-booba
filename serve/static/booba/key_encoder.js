/** Encode one complete legacy key press. Returns null for unsupported gestures/modes. */
export function encodeKey(gesture, modes) {
    if (modes.extendedKeyboard)
        return null;
    const { key, modifiers } = gesture;
    if (new Set(modifiers).size !== modifiers.length ||
        modifiers.some(m => !['shift', 'alt', 'ctrl'].includes(m)))
        return null;
    const shift = modifiers.includes('shift');
    const alt = modifiers.includes('alt');
    const ctrl = modifiers.includes('ctrl');
    const mod = 1 + Number(shift) + 2 * Number(alt) + 4 * Number(ctrl);
    const cursor = { up: 'A', down: 'B', right: 'C', left: 'D', home: 'H', end: 'F' };
    if (Object.prototype.hasOwnProperty.call(cursor, key))
        return mod === 1
            ? `\x1b${modes.applicationCursor ? 'O' : '['}${cursor[key]}`
            : `\x1b[1;${mod}${cursor[key]}`;
    const functions = { f1: 'P', f2: 'Q', f3: 'R', f4: 'S' };
    if (Object.prototype.hasOwnProperty.call(functions, key))
        return mod === 1 ? `\x1bO${functions[key]}` : `\x1b[1;${mod}${functions[key]}`;
    const tilde = { insert: 2, delete: 3, pageup: 5, pagedown: 6,
        f5: 15, f6: 17, f7: 18, f8: 19, f9: 20, f10: 21, f11: 23, f12: 24 };
    if (Object.prototype.hasOwnProperty.call(tilde, key))
        return `\x1b[${tilde[key]}${mod === 1 ? '' : `;${mod}`}~`;
    let text;
    switch (key) {
        case 'enter':
            if (shift || ctrl)
                return null;
            text = modes.newline ? '\r\n' : '\r';
            break;
        case 'escape':
            if (shift || ctrl)
                return null;
            text = '\x1b';
            break;
        case 'tab':
            if (ctrl)
                return null;
            text = shift ? '\x1b[Z' : '\t';
            break;
        case 'backspace':
            if (shift)
                return null;
            text = (Boolean(modes.backarrow) !== ctrl) ? '\b' : '\x7f';
            break;
        default: {
            text = key === 'space' ? ' ' : key;
            if ([...text].length !== 1 || /[\u0000-\u001f\u007f-\u009f\ud800-\udfff]/u.test(text))
                return null;
            if (shift) {
                // A gesture names a character, not a physical keyboard position.
                // Only ASCII letter case has a layout-independent shift mapping.
                if (!/^[a-zA-Z ]$/.test(text))
                    return null;
                text = text.toUpperCase();
            }
            if (ctrl) {
                if (/^[a-zA-Z]$/.test(text))
                    text = String.fromCharCode(text.toUpperCase().charCodeAt(0) - 64);
                else if (' @2'.includes(text))
                    text = '\x00';
                else if ('[\\]^_'.includes(text))
                    text = String.fromCharCode(text.charCodeAt(0) - 64);
                else if (text === '?')
                    text = '\x7f';
                else
                    return null;
            }
        }
    }
    return (alt ? '\x1b' : '') + text;
}
