package gantry

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// ArgSpec declares one custom app argument from gantry.json's "args"
// map. The generated gantry_args.go registers the app's specs via
// SetArgSpecs; each value resolves from its environment variable
// (which gantry dev sets from the command line), falling back to the
// declared default. A bad value in the environment never crashes a
// shipped app - it logs and the default applies.
type ArgSpec struct {
	Name        string // flag name, lowercase-kebab ("api-host")
	Type        string // "string" | "bool" | "int"
	Env         string // environment variable ("GANTRY_ARG_API_HOST")
	Description string
	Default     any
}

var argSpecs []ArgSpec

// SetArgSpecs registers the app's declared args. gantry dev/build/gen
// write a gantry_args.go calling this from init(); calling it by hand
// works too.
func SetArgSpecs(specs []ArgSpec) { argSpecs = specs }

// resolve returns the spec's runtime value: the parsed environment
// variable when present and valid, else the declared default.
func (s ArgSpec) resolve() any {
	raw, ok := os.LookupEnv(s.Env)
	if !ok {
		return s.Default
	}
	switch s.Type {
	case "bool":
		v, err := strconv.ParseBool(raw)
		if err != nil {
			log.Printf("gantry: arg %s: %s=%q is not a bool, using default %v", s.Name, s.Env, raw, s.Default)
			return s.Default
		}
		return v
	case "int":
		v, err := strconv.Atoi(raw)
		if err != nil {
			log.Printf("gantry: arg %s: %s=%q is not an int, using default %v", s.Name, s.Env, raw, s.Default)
			return s.Default
		}
		return v
	default:
		return raw
	}
}

func findSpec(name string) (ArgSpec, bool) {
	for _, s := range argSpecs {
		if s.Name == name {
			return s, true
		}
	}
	return ArgSpec{}, false
}

// Arg returns a declared arg's value as a string (non-string types are
// formatted). Undeclared names return "".
func Arg(name string) string {
	s, ok := findSpec(name)
	if !ok {
		return ""
	}
	return fmt.Sprint(s.resolve())
}

// ArgBool returns a declared bool arg's value; false for undeclared
// names or non-bool args.
func ArgBool(name string) bool {
	s, ok := findSpec(name)
	if !ok {
		return false
	}
	v, _ := s.resolve().(bool)
	return v
}

// ArgInt returns a declared int arg's value; 0 for undeclared names or
// non-int args.
func ArgInt(name string) int {
	s, ok := findSpec(name)
	if !ok {
		return 0
	}
	v, _ := s.resolve().(int)
	return v
}

// Args returns every declared arg resolved to its runtime value -
// declared args only, never the raw process environment. This is what
// the built-in "gantry" service's env call hands the frontend.
func Args() map[string]any {
	out := make(map[string]any, len(argSpecs))
	for _, s := range argSpecs {
		out[s.Name] = s.resolve()
	}
	return out
}
