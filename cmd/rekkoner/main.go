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
	// Find k8s objects in files.
	allObjs := []unstructured.Unstructured{}
	fileCount := 0 // just for informational purposes
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
		if len(objs) > 0 {
			fileCount++
		}
		allObjs = append(allObjs, objs...)
		return nil
	}
	if err := filepath.Walk(os.Args[1], walkFunc); err != nil {
		log.Fatalf("error: %s\n", err)
	}

	// Doing some grouping.
	objsGroupByKind := map[string][]unstructured.Unstructured{}
	for _, obj := range allObjs {
		kind := obj.Object["kind"].(string)
		objsGroupByKind[kind] = append(objsGroupByKind[kind], obj)
	}

	// Range over our discoveries and print some summary.
	fmt.Printf("Found %d objects in %d files:\n", len(allObjs), fileCount)
	for _, persp := range perspectiveCfg {
		fmt.Printf("  % 22s : %d\n", persp.Kind, len(objsGroupByKind[persp.Kind]))
		// TODO handle unknowns
	}
}

var perspectiveCfg = []Perspective{
	{"Namespace", ""},
	{"Service", ""},
	{"Deployment", ""},
	{"ConfigMap", ""},
	{"Ingress", ""},
	{"StatefulSet", ""},
	{"PersistentVolumeClaim", ""},
	{"PersistentVolume", ""},
}

// Perspective configures how we see certain Kinds of k8s object.
// We use it to govern how we print shorthand references to it,
// which fields we diff aggressively vs ignore, etc.
type Perspective struct {
	Kind              string // kind name.  CamelCase, as in k8s.
	ShortnameTemplate string
}
