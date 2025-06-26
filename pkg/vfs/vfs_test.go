package vfs

import (
	"testing"
)

func TestHttpFs(t *testing.T) {
	fs := NewHttpFS("https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/logs/periodic-ci-openshift-release-master-okd-scos-4.20-upgrade-from-okd-scos-4.19-e2e-aws-ovn-upgrade/1937465859354136576/artifacts/e2e-aws-ovn-upgrade/gather-must-gather/artifacts/must-gather/inspect.local.3813551405126079732")
	files, err := fs.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	// clearly, this is broken.
	for i, file := range files {
		t.Logf("files[%d]: %v", i, file.Name())
	}

	file, err := fs.ReadFile("event-filter.html")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("file: %v", string(file))

	file, err = fs.ReadFile("namespaces/kube-system/kube-system.yaml")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("file: %v", string(file))
}
