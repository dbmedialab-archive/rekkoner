package k8s

import (
	"bytes"
	"testing"
)

func TestFileDecoding(t *testing.T) {
	f := bytes.NewBufferString(`apiVersion: v1
kind: Namespace
metadata:
  name: "{{env \"WERCKER_GIT_REPOSITORY\"}}"
  labels:
    environment: "{{env \"ENV\"}}"
`)
	objs, err := DecodeObjects(f)
	if err != nil {
		t.Errorf("%s", err)
	}
	t.Logf("parsed as: %#v\n", objs)
}
