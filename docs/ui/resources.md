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

`resourceURL(name)` turns a resource path into its URL - it is a thin, safe join: `resourceURL("img/logo.png")` returns `/resources/img/logo.png`, and it strips any leading slashes off `name` so `"/img/logo.png"` and `"img/logo.png"` behave the same. Use the returned URL anywhere a URL goes:

```tsx
import { resourceURL } from "gantry-web";

<img src={resourceURL("img/logo.png")} />;

const cfg = await fetch(resourceURL("data/config.json")).then((r) => r.json());
```

In CSS, reference the fixed `/resources/` path directly (there is no build-time rewriting of URLs in CSS) - e.g. a font face:

```css
@font-face {
  font-family: "Inter";
  src: url("/resources/fonts/inter.woff2") format("woff2");
}
```

## Use it from Go

The paired `.go` side reads the same files with no HTTP round trip, through the `gantry` package:

- `gantry.Resource(name string) ([]byte, error)` returns one file's bytes by slash path. It returns `fs.ErrNotExist` when the app has no `resources/` directory *or* the file is absent - it never panics.
- `gantry.Resources() fs.FS` returns the whole tree as an `fs.FS`, for streaming or handing to anything that takes a filesystem. When the app has no `resources/` directory this is an empty filesystem (an internal `emptyFS` whose `Open` always returns `fs.ErrNotExist`), which is what makes both functions always safe to call.

```go
import (
    "html/template"
    "github.com/BlueBeard63/Gantry/gantry"
)

b, err := gantry.Resource("data/config.json")
// or hand the tree to anything that wants an fs.FS:
tmpl := template.Must(template.ParseFS(gantry.Resources(), "email/*.tmpl"))
```

## How it is built and served

- **Embedding.** When your app has a `resources/` directory, `gantry gen`/`dev`/`build` generate `gantry_resources.go`. It `//go:embed all:resources`, calls `fs.Sub` to strip the `resources/` prefix, and registers the tree in an `init()` via `gantry.SetResources`. No directory (or an empty one) means no generated file and an empty tree - and any stale generated file is removed so the app still compiles (an `//go:embed` with nothing to match is a build error). **Never edit the generated file;** add or remove files under `resources/` and let gantry regenerate it. (The `all:` prefix means dotfiles and `_`-prefixed files are embedded too.)
- **Serving in a built app.** The Go side mounts the embedded tree with `http.FileServer(http.FS(...))` under `http.StripPrefix("/resources/", ...)`, registered *before* the SPA catch-all `/` so `/resources/*` is never swallowed into `index.html`.
- **Serving in `gantry dev`.** The Vite plugin serves `/resources/<path>` live off `<appRoot>/resources` on disk - the same tree, but with edits visible on refresh without a rebuild, and path-traversal (`../`) rejected. It is deliberately kept off the Go proxy so a resource edit does not need the Go app to restart. The URL is identical either way, which is why `resourceURL` is all your code ever needs.

## Notes

- Paths are slash-separated and relative to `resources/`: `"img/logo.png"`, not `"/img/logo.png"` or a filesystem path. `resourceURL` strips a leading slash for you; `gantry.Resource` expects the clean relative form.
- Resources are read-only and baked into the binary - they are for assets that ship *with* the app, not for user data you write at runtime.
- You can register a tree yourself with `gantry.SetResources(fsys)` (any `fs.FS`) when wiring an app up [without the CLI](../advanced/without-the-cli.md); the generated file is just the CLI doing this for you.

Next: [Styling](styling.md) for referencing fonts and images from CSS, or [Project structure](../getting-started/project-structure.md) for where `resources/` sits among the app's folders.
