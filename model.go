package rekkoner

import (
	"sort"

	. "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// IntentPath is a string to the source file of a k8s object in our user's
// declared intents.  It may be suffixed with a bang and
// number -- e.g. "!%d" -- if there was more than one object in that file.
//
// We generally do all operations sorted by IntentPath.
// This is similar to behaviors in the kubectl tool.
//
// K8s objects that are observed from the live cluster state rather than
// intent files do not have an IntentPath.
type IntentPath string

// IntentPathByString provides a sortable interface.
type IntentPathByString []IntentPath

func (p IntentPathByString) Len() int           { return len(p) }
func (p IntentPathByString) Less(i, j int) bool { return p[i] < p[j] }
func (p IntentPathByString) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Intent contains the deserialized user's k8s config file content...
// whatever that is; it may be any Kind, and we keep things unstructured.
type Intent struct {
	Objs map[IntentPath]Unstructured
	Keys []IntentPath
}

func (_ Intent) Init() Intent {
	return Intent{
		Objs: make(map[IntentPath]Unstructured),
	}
}
func (itt *Intent) Sync() {
	itt.Keys = make([]IntentPath, 0, len(itt.Objs))
	for k := range itt.Objs {
		itt.Keys = append(itt.Keys, k)
	}
	sort.Sort(IntentPathByString(itt.Keys))
}
