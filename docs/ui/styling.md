# Styling

CSS in Gantry follows the same convention as everything else: files live next to what they style, one root file owns the theme, and the chrome and Tea built-ins read that theme from CSS variables so a retheme is new values, not new components.

## The three levels

1. **Root `index.css`** - app-wide. Loaded first, applies everywhere. This is where the theme variables live.
2. **`pages/<name>/<name>.css`** - one page's styles.
3. **`components/<name>/<name>.css`** and **`layouts/<name>/<name>.css`** - one component's or layout's styles.

Colocated CSS is imported automatically by the Vite plugin - no `import` statement in the `.tsx`, no central stylesheet list to maintain. Create the file next to the `.tsx` and it is live (`gantry dev` picks up new files on the fly).

## Scoping by convention

All CSS is loaded globally (that is how CSS works), so scope your selectors. Every page renders inside a wrapper element carrying two classes: the shared `gantry-page` and a stable per-page class derived from its key by replacing every run of non-alphanumeric characters with a hyphen:

```
pages/index      -> .gantry-pages-index
pages/settings   -> .gantry-pages-settings
pages/account/settings -> .gantry-pages-account-settings   (key "pages/account/settings")
```

Start a page's rules with that class:

```css
.gantry-pages-settings .field-row {
  display: flex;
  gap: 8px;
}
```

Components and layouts are not wrapped in a framework-generated class, so give the component's own root element a class named after it and scope under that (the scaffold's example component shows the pattern):

```css
.example-card { /* ... */ }
```

## Dynamic routes

A [dynamic page](dynamic-routes.md) (`[id]`, `[...slug]`) is a single component that serves many URLs, and it keeps **one stable wrapper class** across all of them - so one CSS block styles every instance. The class comes from the same rule as any other page (the key with every run of non-alphanumeric characters replaced by a hyphen), applied to the bracketed key. The brackets are non-alphanumeric, so they collapse into the hyphen runs - and because the key *ends* with `]`, the derived class ends with a trailing hyphen. Write your selectors with that trailing hyphen exactly as generated:

```
pages/examples/page1/[id]  -> .gantry-pages-examples-page1-id-
pages/files/[...slug]      -> .gantry-pages-files-slug-
```

```css
/* pages/examples/page1/[id]/[id].css - note the trailing hyphen */
.gantry-pages-examples-page1-id- .item {
  padding: 8px;
}
```

Colocate the CSS with the page like any other pair - `pages/examples/page1/[id]/[id].css` next to `[id].tsx` - and the Vite plugin imports it automatically. The wrapper class does not change as the captured `id`/`slug` changes (only the params do, see [Dynamic routes](dynamic-routes.md)), so a single rule set covers `/examples/page1/1`, `/examples/page1/2`, and every other address the page serves.

## The theme variables

The window chrome and the Tea built-ins read their colors and metrics from CSS custom properties, all prefixed `--gantry-`. They are defined in `:root` inside gantry-web's `styles.css`; redefine any of them in your root `index.css` and the whole app rethemes - TitleBar included, no component changes. The full set:

```css
:root {
  --gantry-bg: #101012;              /* app background */
  --gantry-fg: #e8e8ea;              /* primary text */
  --gantry-fg-dim: #9a9aa0;          /* secondary / dimmed text */
  --gantry-accent: #6ea8fe;          /* focus, highlights, progress fill */
  --gantry-border: #2a2a2e;          /* dividers, outlines */
  --gantry-titlebar-h: 40px;         /* default caption height (var form) */
  --gantry-titlebar-bg: transparent; /* titlebar background */
  --gantry-titlebar-fg: var(--gantry-fg); /* titlebar text + window buttons */
  --gantry-btn-hover: rgba(255,255,255,0.08); /* window/tb button hover */
  --gantry-close-hover: #c42b1c;     /* close button hover */
  --gantry-control-bg: #1a1a1e;      /* buttons, inputs, selects */
  --gantry-control-border: #34343a;
  --gantry-radius: 6px;
  --gantry-font: "Segoe UI", system-ui, -apple-system, sans-serif;
}
```

A few more variables are *consumed with fallbacks* but not defined in `:root`, so they exist only if you set them: `--gantry-mono` (monospace family for error stacks and codes, falls back to `ui-monospace, monospace`) and the titlebar-button variables `--gantry-tbbtn-w`, `--gantry-tbbtn-bg`, `--gantry-tbbtn-fg`, `--gantry-tbbtn-hover` (see [TitleBar](titlebar.md)).

Your own styles can - and should - use the same variables so custom UI tracks the theme:

```css
.gantry-pages-index .stat-card {
  background: var(--gantry-control-bg);
  border: 1px solid var(--gantry-border);
  border-radius: var(--gantry-radius);
}
```

## Plain elements are themed for free

Inside the app scaffold (`.gantry-app`), gantry-web already styles ordinary `<input>`, `<textarea>`, `<select>`, and `<button>` (excluding the window and titlebar buttons) with the control variables above, plus `accent-color` on checkboxes and a pointer cursor on `Link`'s hrefless anchors. So a page can write plain HTML controls and get the app look with no per-page CSS - the Tea built-ins carry the same values, so the two match.

## Styling Tea built-ins

Every [Tea built-in](tea-nodes.md) carries a stable class: `.gantry-tea-column`, `.gantry-tea-row`, `.gantry-tea-text`, `.gantry-tea-heading`, `.gantry-tea-button`, `.gantry-tea-input`, `.gantry-tea-checkbox`, `.gantry-tea-select`, `.gantry-tea-divider`, `.gantry-tea-spacer`, `.gantry-tea-progress` (with an inner `.gantry-tea-progress-fill`), and `.gantry-tea-unknown` for an unresolved `Custom` name. Add your own hook with the Go-side `"class"` prop and scope under the page:

```go
ui.Button("Save", saveMsg{}).WithProps("class", "primary")
```

```css
.gantry-pages-settings .gantry-tea-button.primary {
  background: var(--gantry-accent);
  color: #08131f;
}
```

The other Go-side layout hints become inline styles: `"gap"` -> `gap` (px), `"pad"` -> `padding` (px), `"grow": true` -> `flex-grow: 1`. Use those for one-offs and CSS classes for anything reused.

## Overriding variables per page

Variables cascade, so a page (or any element) can override a `--gantry-*` value and every built-in inside it picks the override up automatically - no component change:

```css
/* pages/zen/zen.css - this page only, a calmer accent */
.gantry-pages-zen {
  --gantry-accent: #8bc48a;
}
```

Root `index.css` loads before the colocated files, so a page's CSS comes later in the cascade and can override root values - including these variables - for its own subtree.

## Advanced: Tailwind

Nothing in gantry-web requires Tailwind - the chrome ships plain CSS, and the variables-plus-plain-CSS convention carries a long way. Tailwind v4 is supported first-class when you want it:

- **New app**: `gantry new myapp --tailwind`. The generated `index.css` becomes a Tailwind `@theme` token file: every color is both a utility class (`bg-surface`, `text-primary`, `border-border-subtle`, ...) and a real CSS custom property (`var(--color-surface)`), and the `--gantry-*` chrome variables alias those tokens (`--gantry-bg: var(--color-base)`), so retheming the app and retheming the chrome are one edit.
- **Existing app**: `gantry install --tailwind` installs the packages, migrates your `index.css` into that shape (you review the diff), sets `"tailwind": true` in `gantry.json`, and regenerates the synthesized Vite config with `@tailwindcss/vite` included.

The `"tailwind"` flag in `gantry.json` is what makes `gantry dev`/`build` emit the plugin into `.gantry/vite.config.ts` - you never own the Vite config for Tailwind. [Without the CLI](../advanced/without-the-cli.md) is the path for other build customizations.
