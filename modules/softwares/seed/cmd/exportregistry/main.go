package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/izetmolla/containerws/modules/softwares/seed"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: exportregistry <output-dir>")
		os.Exit(2)
	}
	res, err := seed.ExportRegistryWithTimeout(os.Args[1], 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
}
