package cmd

import (
	"log"
	"os"
	"path/filepath"
)

func Execute() {
	// Help text follows how the binary was invoked (cws or containerws).
	switch name := filepath.Base(os.Args[0]); name {
	case "cws", "containerws":
		rootCmd.Use = name
	}
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
