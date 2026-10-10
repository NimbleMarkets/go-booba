# Host-Styled Browser Chrome

**Date:** 2026-10-09  
**Status:** Design for the experimental Terminal Actions bar; applies to any future booba-owned DOM  
**Scope:** `ts/action_bar.ts`, `ts/booba.ts`, the actions demo page, and the usage docs

booba is an embeddable library. The host page owns the look of everything around the terminal canvas, including the Terminal Actions bar. booba supplies the structure and a few functional defaults, and the host restyles any of it with ordinary CSS. No build step, no theme object for DOM chrome, and no `!important`.

## Problem

The action bar is the first booba-owned DOM beyond the canvas. Today it sets one class on its root and applies layout, touch-target size, and `touch-action` as inline styles. Inline declarations beat every host selector, so a host that wants denser buttons on desktop writes a rule and watches it lose. Colors, borders, and fonts are not set inline, so those do respond to host CSS. The split is accidental and undocumented, and the demo page duplicates the terminal theme's hex colors to make its chrome match.

## Design

### Class contract

Every element the bar creates carries a stable class. Modifiers follow the `block--modifier` convention already common in host stylesheets.

| Element | Class | Notes |
| --- | --- | --- |
| Root container | `booba-action-bar` | Unchanged. `role="group"`, `aria-label="Terminal actions"`. |
| Group of related buttons | `booba-action-group` | One per short or full help group. `role="group"`. |
| Any button in the bar | `booba-action` | Action buttons, the More/Less toggle, and the Keyboard button. |
| More/Less toggle | `booba-action--toggle` | In addition to `booba-action`. State is the existing `aria-expanded`. |
| Keyboard button | `booba-action--keyboard` | In addition to `booba-action`. |

State is expressed through attributes the bar already sets, so hosts style `.booba-action:disabled` and `.booba-action--toggle[aria-expanded="true"]` without new hooks. `data-action-focus` remains internal and is documented as such.

No element receives inline styles.

### Default stylesheet in a cascade layer

The bar's functional defaults move into one stylesheet wrapped in `@layer booba`:

```css
@layer booba {
  .booba-action-bar,
  .booba-action-group { display: flex; flex-wrap: wrap; gap: 0.5rem; }
  .booba-action-bar { align-items: center; }
  .booba-action { min-height: 44px; min-width: 44px; touch-action: manipulation; }
}
```

Unlayered author styles always beat layered ones regardless of specificity, so a host rule as plain as `.booba-action { min-height: 36px }` wins. Hosts that use layers themselves order `booba` first with `@layer booba, app;`.

`installActionBar` injects this stylesheet once per document as a `<style data-booba-styles>` element in `<head>`. The check is on the attribute, so several bars, or repeated install and dispose cycles, share one element. The element is never removed; it is inert without bar markup.

The CSS text is exported as `ACTION_BAR_CSS` and the option `styles?: boolean` (default `true`) disables injection. Together they serve hosts whose Content Security Policy forbids inline styles: they set `styles: false` and serve the exported text from their own stylesheet.

### Theme bridge to CSS custom properties

The canvas is painted from the `theme` option and cannot be styled with CSS. The DOM around it can. After `init()` opens the terminal, `BoobaTerminal` sets `--booba-background` and `--booba-foreground` on the container element from the resolved theme, when those values are present. Custom properties inherit downward, so they are visible to the container and its descendants, such as the padding gutter the host draws around the canvas, and not to sibling chrome. Host CSS references them with `var(--booba-background)` so that gutter matches the canvas without repeating hex values.

Only these two properties are exposed. The sixteen-color palette and cursor colors stay in the theme object until a host needs them. The properties are set once at init; booba has no runtime theme-change API today, so there is nothing to keep in sync.

### Demo page

`cmd/booba-actions-example/browser.html` is updated to show the contract in use: a `.booba-action` rule that overrides a layered default, and `var(--booba-background)` on the terminal container in place of the duplicated hex. The page keeps its own global `button` rule for its Restart button.

### Documentation

`docs/TERMINAL_ACTIONS.md` gains a "Styling" subsection under "Browser controls" covering the class table, the layer, the `styles` option with `ACTION_BAR_CSS`, and the two custom properties. `docs/TYPESCRIPT_API.md` notes the custom properties next to the `theme` option.

## Out of scope

Shadow DOM, a `theme` object for DOM chrome, exposing the full palette as custom properties, a shipped `.css` file in `booba-assets`, and changes to the page-level `<style>` blocks in `serve/static/index.html` or the `booba-assets` template. Those blocks belong to pages, not to the library.

## Testing

Vitest with happy-dom, alongside the existing TypeScript tests.

- `installActionBar` applies the class contract: root, group, every button, and the two modifiers. No created element has a `style` attribute.
- Installing two bars in one document produces exactly one `<style data-booba-styles>`; installing after dispose does not add another.
- `styles: false` injects nothing; `ACTION_BAR_CSS` contains `@layer booba` and the four selectors above.
- `BoobaTerminal.init()` leaves `--booba-background` and `--booba-foreground` on the container, read back through `style.getPropertyValue`, and omits a property whose theme value is absent.

Compiled assets under `serve/static/booba/` are regenerated with `npm run build:embed` and committed with the source change.
