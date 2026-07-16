# Styling

CSS in Gantry follows the same convention as everything else: files
live next to what they style, and one root file owns the theme.

## The three levels

1. Root index.css - app-wide. Loaded first, applies everywhere. This
   is where the theme variables live.
2. pages/<name>/<name>.css - one page's styles.
3. components/<name>/<name>.css - one component's styles.

Colocated css is imported automatically by the Vite plugin - no import
statement in the tsx, no central stylesheet list to maintain. Create
the file next to the tsx and it is live (gantry dev picks up new files
on the fly).

## Scoping by convention

All css is globally loaded (that is how css works), so scope your
selectors. Every page renders inside an element with a stable class
derived from its key:

```
pages/index      -> .gantry-pages-index
pages/settings   -> .gantry-pages-settings
```

Start page rules with that class:

```css
.gantry-pages-settings .field-row {
  display: flex;
  gap: 8px;
}
```

Components wrap themselves: give your component's root element a class
named after it and scope under that (the scaffold's example component
shows the pattern):

```css
.example-card { /* ... */ }
```

## The theme variables

The window chrome and the Tea built-ins read their colors and metrics
from CSS variables, all prefixed --gantry-. Redefine them in your root
index.css and the whole app rethemes - TitleBar included, no component
changes:

```css
:root {
  --gantry-bg: #101012;            /* app background */
  --gantry-fg: #e8e8ea;            /* text */
  --gantry-fg-dim: #9a9aa0;        /* secondary text */
  --gantry-accent: #6ea8fe;        /* focus, highlights, progress */
  --gantry-border: #2a2a2e;        /* dividers, outlines */
  --gantry-titlebar-bg: transparent;
  --gantry-btn-hover: rgba(255, 255, 255, 0.08);
  --gantry-close-hover: #c42b1c;   /* close button hover */
  --gantry-control-bg: #1a1a1e;    /* buttons, inputs */
  --gantry-control-border: #34343a;
  --gantry-radius: 6px;
  --gantry-font: "Segoe UI", system-ui, sans-serif;
}
```

Your own styles can (and should) use the same variables so custom UI
follows the theme:

```css
.gantry-pages-index .stat-card {
  background: var(--gantry-control-bg);
  border: 1px solid var(--gantry-border);
  border-radius: var(--gantry-radius);
}
```

## Styling Tea built-ins

Every built-in carries a stable class: .gantry-tea-button,
.gantry-tea-input, .gantry-tea-checkbox, .gantry-tea-select,
.gantry-tea-progress, .gantry-tea-column, .gantry-tea-row, and so on.
Combine with the Go-side "class" prop for targeted styling:

```go
ui.Button("Save", saveMsg{}).WithProps("class", "primary")
```

```css
.gantry-pages-settings .gantry-tea-button.primary {
  background: var(--gantry-accent);
  color: #08131f;
}
```

Layout hints from Go ("gap", "pad", "grow") become inline styles; use
them for one-offs and css classes for anything reused.

## Load order

Root index.css loads first, then every colocated css. Since page css
comes later, a page can override root values - including the variables:

```css
/* pages/zen/zen.css - this page only, calmer accent */
.gantry-pages-zen {
  --gantry-accent: #8bc48a;
}
```

Variables cascade, so built-ins inside that page pick the override up
automatically.

## Tailwind and friends

Nothing in gantry-web requires Tailwind - the chrome ships plain css.
If you want Tailwind, add it to the app (gantry add tailwindcss
@tailwindcss/vite) and note you would need to extend the synthesized
vite config, which means graduating to
[Without the CLI](../advanced/without-the-cli.md). For most apps the
variables + plain css convention carries a long way.
