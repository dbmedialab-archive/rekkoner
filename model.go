package rekkoner

import (
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

// Intent contains the deserialized user's k8s config file content...
// whatever that is; it may be any Kind, and we keep things unstructured.
type Intent struct {
	Objs map[IntentPath]Unstructured
}

func (_ Intent) Init() Intent {
	return Intent{
		make(map[IntentPath]Unstructured),
	}
}
