package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BlueBeard63/Gantry/gerr"
)

// The arg harness: gantry.json's "args" map declares custom app
// arguments; gantry dev registers them as real flags (validation and
// --help for free) and hands the values to the app as environment
// variables. Production binaries read the same variables directly,
// against the spec baked in by the generated gantry_args.go.

var (
	argNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	argEnvRe  = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

// reservedArgNames are flags gantry dev or gantry.Run already own; a
// declared arg must not shadow them.
var reservedArgNames = map[string]bool{
	"vite-port": true, "port": true, "dev-url": true, "browser": true,
	"no-open": true, "tray": true, "no-tray": true, "token": true,
	"announce-ready": true, "emit-widgets": true, "shellrole": true,
	"url": true, "monitor": true, "position": true, "help": true, "h": true,
}

// validateArgSpecs rejects a gantry.json args block the harness cannot
// honor, with the exact key that is wrong.
func validateArgSpecs(cfg appConfig) error {
	for name, spec := range cfg.Args {
		if !argNameRe.MatchString(name) {
			return gerr.New("config.bad-arg-spec", "args.%s: bad arg name", name).
				WithHint("arg names are lowercase kebab-case: ^[a-z][a-z0-9-]*$")
		}
		if reservedArgNames[name] {
			return gerr.New("config.bad-arg-spec", "args.%s: name is reserved by gantry's own flags", name)
		}
		switch spec.Type {
		case "", "string", "bool", "int":
		default:
			return gerr.New("config.bad-arg-spec", "args.%s: unknown type %q (string, bool or int)", name, spec.Type)
		}
		if spec.Env != "" && !argEnvRe.MatchString(spec.Env) {
			return gerr.New("config.bad-arg-spec", "args.%s: bad env name %q", name, spec.Env).
				WithHint("env names are UPPER_SNAKE: ^[A-Z][A-Z0-9_]*$")
		}
		if _, _, err := argDefault(spec); err != nil {
			return gerr.Wrap("config.bad-arg-spec", err, "args.%s", name)
		}
	}
	return nil
}

// argDefault parses the spec's JSON default into the typed value and
// its string form (the env var payload). A missing default yields the
// type's zero value.
func argDefault(spec argSpec) (val any, str string, err error) {
	typ := spec.Type
	if typ == "" {
		typ = "string"
	}
	if spec.Default == nil {
		switch typ {
		case "bool":
			return false, "false", nil
		case "int":
			return 0, "0", nil
		default:
			return "", "", nil
		}
	}
	switch typ {
	case "bool":
		var v bool
		if err := json.Unmarshal(spec.Default, &v); err != nil {
			return nil, "", fmt.Errorf("default %s is not a bool", spec.Default)
		}
		return v, strconv.FormatBool(v), nil
	case "int":
		var v int
		if err := json.Unmarshal(spec.Default, &v); err != nil {
			return nil, "", fmt.Errorf("default %s is not an int", spec.Default)
		}
		return v, strconv.Itoa(v), nil
	default:
		var v string
		if err := json.Unmarshal(spec.Default, &v); err != nil {
			return nil, "", fmt.Errorf("default %s is not a string", spec.Default)
		}
		return v, v, nil
	}
}

// argEnvName is the environment variable one declared arg travels in:
// the explicit "env" name, or GANTRY_ARG_<UPPER_SNAKE> from the flag
// name (api-host -> GANTRY_ARG_API_HOST).
func argEnvName(name string, spec argSpec) string {
	if spec.Env != "" {
		return spec.Env
	}
	return "GANTRY_ARG_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// sortedArgNames returns the declared arg names in stable order for
// flag registration, --help and codegen.
func sortedArgNames(cfg appConfig) []string {
	names := make([]string, 0, len(cfg.Args))
	for n := range cfg.Args {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// registerArgFlags adds one real flag per declared arg to the dev
// FlagSet - stdlib flag then does --name=value parsing, unknown-flag
// errors and -h listing. The returned getters yield each arg's final
// string value (the env var payload) after Parse.
func registerArgFlags(fs *flag.FlagSet, cfg appConfig) map[string]func() string {
	getters := make(map[string]func() string, len(cfg.Args))
	for _, name := range sortedArgNames(cfg) {
		spec := cfg.Args[name]
		def, _, _ := argDefault(spec) // validated earlier
		usage := spec.Description
		if usage == "" {
			usage = "app arg (gantry.json)"
		}
		switch spec.Type {
		case "bool":
			p := fs.Bool(name, def.(bool), usage)
			getters[name] = func() string { return strconv.FormatBool(*p) }
		case "int":
			p := fs.Int(name, def.(int), usage)
			getters[name] = func() string { return strconv.Itoa(*p) }
		default:
			p := fs.String(name, def.(string), usage)
			getters[name] = func() string { return *p }
		}
	}
	return getters
}

// argEnv renders the declared args as environment assignments for the
// spawned app process.
func argEnv(cfg appConfig, getters map[string]func() string) []string {
	out := make([]string, 0, len(getters))
	for _, name := range sortedArgNames(cfg) {
		out = append(out, argEnvName(name, cfg.Args[name])+"="+getters[name]())
	}
	return out
}

// devUsage prints gantry dev's help: the built-in flags plus the app's
// declared args from gantry.json.
func devUsage(cfg appConfig) func() {
	return func() {
		fmt.Println("Usage: gantry dev [flags] [-- app flags]")
		fmt.Println("\nFlags:")
		fmt.Println("  --vite-port int   vite dev server port (default 5173)")
		if len(cfg.Args) > 0 {
			fmt.Println("\nApp args (declared in gantry.json, passed as env vars):")
			for _, name := range sortedArgNames(cfg) {
				spec := cfg.Args[name]
				typ := spec.Type
				if typ == "" {
					typ = "string"
				}
				_, def, _ := argDefault(spec)
				line := fmt.Sprintf("  --%s %s   %s", name, typ, spec.Description)
				line = strings.TrimRight(line, " ")
				fmt.Printf("%s (default %s, env %s)\n", line, def, argEnvName(name, spec))
			}
		}
		fmt.Println("\nEverything after -- is passed to the app process unchanged.")
	}
}
