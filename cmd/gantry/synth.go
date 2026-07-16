package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeSynth regenerates the .gantry/ Vite build root. These files are
// synthesized on every dev/build run and never hand-edited - the app's
// own files are main.go, pages/, components/, index.css and package.json.
func writeSynth(appDir string, cfg appConfig) (string, error) {
	dir := filepath.Join(appDir, ".gantry")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	indexHTML := fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>%s</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/main.tsx"></script>
  </body>
</html>
`, cfg.Title)

	mainTSX := fmt.Sprintf(`// Synthesized by gantry - regenerated on every dev/build run.
import { createApp } from "gantry-web";

createApp({ title: %q });
`, cfg.Title)

	viteConfig := `// Synthesized by gantry - regenerated on every dev/build run.
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { gantry } from "gantry-web/vite";

export default defineConfig({
  plugins: [react(), gantry({ appRoot: ".." })],
  build: { outDir: "../dist", emptyOutDir: true },
});
`

	files := map[string]string{
		"index.html":     indexHTML,
		"main.tsx":       mainTSX,
		"vite.config.ts": viteConfig,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}
