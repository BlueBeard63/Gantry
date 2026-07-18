# Resources

Anything you drop in your app's `resources/` directory - images, fonts, JSON, any static file - is embedded into the binary once and served at `/resources/<path>`. Both halves of the app read the same bytes: the frontend by URL, the Go side by path. There is nothing to configure; the folder is the whole feature.

## Add a resource

Make a `resources/` directory at the app root and put files in it, in whatever subfolders you like:

```
resources/
  img/logo.png
  fonts/inter.woff2
  data/config.json
```

That is the entire setup. `gantry dev`/`build` notice the directory and wire it up (see [How it is built and served](#how-it-is-built-and-served) below).

## Use it from the frontend

`resourceURL(name)` turns a resource path into its URL (`resourceURL("img/logo.png")` returns `/resources/img/logo.png`). Use that URL anywhere a URL goes:

```tsx
import { resourceURL } from "gantry-web";

<img src={resourceURL("img/logo.png")} />;

const cfg = await fetch(resourceURL("data/config.json")).then((r) => r.json());
```

In CSS, reference it the same way - e.g. a font face:

```css
@font-face {
  font-family: "Inter";
  src: url("/resources/fonts/inter.woff2") format("woff2");
}
```

## Use it from Go

The paired `.go` side reads the same files without any HTTP round trip:

- `gantry.Resource("data/config.json")` returns the file's bytes (`[]byte`, `error`). It returns `fs.ErrNotExist` when the app has no `resources/` directory or the file is absent - it never panics.
- `gantry.Resources()` returns the whole tree as an `fs.FS`, for streaming or serving. When the app has no `resources/` directory this is an empty filesystem, so `Resource` and `Resources` are always safe to call.

```go
import "github.com/B-Commissions/Gantry/gantry"

b, err := gantry.Resource("data/config.json")
// or hand the tree to anything that wants an fs.FS:
tmpl := template.Must(template.ParseFS(gantry.Resources(), "email/*.tmpl"))
```

## How it is built and served

- **Embedding.** When your app has a `resources/` directory, `gantry gen`/`dev`/`build` generate `gantry_resources.go`, which `//go:embed`s the tree and registers it with `gantry.SetResources`. No directory means no generated file and an empty tree - nothing breaks. Never edit the generated file; add or remove files under `resources/` and let gantry regenerate it.
- **Serving.** In a built app the Go side serves the embedded tree at `/resources/` (a plain file server). During `gantry dev` the Vite plugin serves the same folder live off disk instead, so edits to a resource show up on refresh without a rebuild - the URL is identical either way, which is why `resourceURL` is all your code ever needs.

## Notes

- Paths are slash-separated and relative to `resources/` (`"img/logo.png"`, not `"/img/logo.png"` or a filesystem path). `resourceURL` strips a leading slash for you.
- Resources are read-only and baked into the binary - they are for assets that ship with the app, not for user data you write at runtime.
- You can register a tree yourself with `gantry.SetResources(fsys)` (any `fs.FS`) if you are wiring an app up [without the CLI](../advanced/without-the-cli.md).

Next: [Styling](styling.md), or [Project structure](../getting-started/project-structure.md) for where `resources/` sits among the app's folders.
