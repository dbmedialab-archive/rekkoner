package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dbmedialab/rekkoner/velcro/k8s"
)

func main() {
	allObjs := []unstructured.Unstructured{}
	walkFunc := func(path string, info os.FileInfo, err error) error {
		// Only interested in plain files.
		if !info.Mode().IsRegular() {
			return nil
		}
		// Do some filtering by name.
		// TODO This should get either more rigorous or more flag-controlled or... something.
		// ECOSYSTEM: issue with secrets files in truthy may or may not be parsible depending on git-filter state fun.
		switch filepath.Ext(path) {
		case ".yaml", ".yml":
			// pass!
		default:
			log.Printf("skipping %q, not yaml extension", path)
			return nil
		}
		// Open file and parse.
		// The k8s parser will error for things like "Object 'Kind' is missing",
		//  so we're fairlyyyy sure we've got sane k8s objects if no error.
		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err != nil {
			log.Printf("error: %q: %s\n", path, err)
		}
		objs, err := k8s.DecodeObjects(f)
		if err != nil {
			log.Printf("error: %q: %s\n", path, err)
		}
		allObjs = append(allObjs, objs...)
		return nil
	}
	if err := filepath.Walk(os.Args[1], walkFunc); err != nil {
		log.Fatalf("error: %s\n", err)
	}
	kindCount := map[string]int{}
	for _, obj := range allObjs {
		kind := obj.Object["kind"].(string)
		kindCount[kind]++
	}
	fmt.Printf("%s\n", kindCount)
}
