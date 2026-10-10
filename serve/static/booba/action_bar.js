/**
 * Functional defaults for the bar: wrapping flex layout, 44px touch targets,
 * and no double-tap zoom. They live in the `booba` cascade layer, so any
 * unlayered host rule such as `.booba-action { min-height: 36px }` wins
 * regardless of specificity and without `!important`.
 */
export const ACTION_BAR_CSS = `@layer booba {
  .booba-action-bar,
  .booba-action-group { display: flex; flex-wrap: wrap; gap: 0.5rem; }
  .booba-action-bar { align-items: center; }
  .booba-action { min-height: 44px; min-width: 44px; touch-action: manipulation; }
}
`;
const STYLE_ATTRIBUTE = 'data-booba-styles';
function ensureStyles(doc) {
    if (doc.head.querySelector(`style[${STYLE_ATTRIBUTE}]`))
        return;
    const style = doc.createElement('style');
    style.setAttribute(STYLE_ATTRIBUTE, '');
    style.textContent = ACTION_BAR_CSS;
    doc.head.append(style);
}
/** Mount accessible controls in a host-owned element; return an idempotent disposer. */
export function installActionBar(controller, options) {
    const doc = options.parent.ownerDocument;
    if (options.styles !== false)
        ensureStyles(doc);
    const root = doc.createElement('div');
    root.className = 'booba-action-bar';
    root.setAttribute('role', 'group');
    root.setAttribute('aria-label', 'Terminal actions');
    options.parent.append(root);
    let expanded = false;
    let disposed = false;
    // Invisible direction overrides in application labels must not reorder host UI.
    const display = (value) => value.replace(/[\u200b-\u200f\u202a-\u202e\u2060-\u206f\ufeff]/gu, '');
    const button = (label, activate, modifier) => {
        const b = doc.createElement('button');
        b.type = 'button';
        b.className = modifier ? `booba-action ${modifier}` : 'booba-action';
        b.textContent = display(label);
        // Avoid moving focus to the terminal textarea or changing the software keyboard.
        b.addEventListener('pointerdown', event => { if (event.pointerType !== 'mouse')
            event.preventDefault(); });
        b.addEventListener('click', () => { if (!disposed)
            activate(); });
        return b;
    };
    const render = () => {
        if (disposed)
            return;
        const focused = document.activeElement instanceof HTMLElement && root.contains(document.activeElement)
            ? document.activeElement.dataset.actionFocus : undefined;
        root.replaceChildren();
        const snapshot = controller.snapshot;
        if (snapshot) {
            const byID = new Map(snapshot.actions.map(a => [a.id, a]));
            const appendAction = (parent, action) => {
                const indexes = action.preferred === undefined ? action.keys.map((_, i) => i) : [action.preferred];
                for (const index of indexes) {
                    const key = action.keys[index];
                    const keyLabel = [...key.modifiers, key.key].join('+');
                    const label = indexes.length === 1 ? action.label : `${action.label} (${keyLabel})`;
                    const b = button(label, controller.capture(action.id, index));
                    b.dataset.actionFocus = `${action.id}:${index}`;
                    b.disabled = !controller.canActivate(action.id, index);
                    b.title = display(action.hint || keyLabel);
                    parent.append(b);
                }
            };
            const groups = expanded ? snapshot.full.map(g => [...g]) : [snapshot.short];
            if (expanded) {
                const included = new Set(groups.flat());
                const extra = snapshot.short.filter(id => !included.has(id));
                if (extra.length)
                    groups.push(extra);
            }
            for (const refs of groups) {
                if (!refs.length)
                    continue;
                const group = doc.createElement('div');
                group.className = 'booba-action-group';
                group.setAttribute('role', 'group');
                for (const id of refs)
                    appendAction(group, byID.get(id));
                root.append(group);
            }
            if (snapshot.full.some(group => group.length)) {
                const toggle = button(expanded ? 'Less' : 'More', () => { expanded = !expanded; render(); }, 'booba-action--toggle');
                toggle.setAttribute('aria-expanded', String(expanded));
                toggle.dataset.actionFocus = 'toggle';
                root.append(toggle);
            }
        }
        if (options.openKeyboard) {
            const keyboard = button('Keyboard', options.openKeyboard, 'booba-action--keyboard');
            keyboard.dataset.actionFocus = 'keyboard';
            root.append(keyboard);
        }
        if (focused)
            Array.from(root.querySelectorAll('button')).find(b => b.dataset.actionFocus === focused)?.focus({ preventScroll: true });
        options.onLayout?.();
    };
    const unsubscribe = controller.subscribe(render);
    render();
    return () => { if (disposed)
        return; disposed = true; unsubscribe(); root.remove(); options.onLayout?.(); };
}
