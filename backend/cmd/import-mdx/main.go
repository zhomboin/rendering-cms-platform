package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	source := flag.String("source", "", "MDX source directory")
	flag.Parse()

	if *source == "" {
		fmt.Fprintln(os.Stderr, "missing -source")
		os.Exit(2)
	}

	var files []string
	err := filepath.WalkDir(*source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".mdx" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan source: %v\n", err)
		os.Exit(1)
	}

	for _, file := range files {
		fmt.Println(file)
	}
}
