// gantry is the Gantry framework CLI.
//
//	gantry new <name>    scaffold a new app
//	gantry dev           run the app with live reload in a native window
//	gantry build         build a single exe with the frontend embedded
//	gantry add <pkg...>  install npm packages into the app
//	gantry docs [topic]  browse the documentation offline
package main

import (
	"fmt"
	"os"
)

const usage = `gantry - build native desktop apps with Go and React

Usage:
  gantry new <name>     scaffold a new app (interactive; see flags below)
  gantry dev            run the current app with live reload
  gantry build          build the current app into a single exe
  gantry add <pkg...>   install npm packages into the app
  gantry docs [topic]   browse the documentation offline

Run gantry new -h for scaffolding flags.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "new":
		err = cmdNew(os.Args[2:])
	case "dev":
		err = cmdDev(os.Args[2:])
	case "build":
		err = cmdBuild(os.Args[2:])
	case "add":
		err = cmdAdd(os.Args[2:])
	case "docs":
		err = cmdDocs(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "gantry: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gantry: %v\n", err)
		os.Exit(1)
	}
}
