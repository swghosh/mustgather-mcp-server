package vfs

import (
	"path"
	"strings"
	"testing"
)

// func TestHttpFs(t *testing.T) {
// 	fs := NewHttpFS("https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/logs/periodic-ci-openshift-release-master-okd-scos-4.20-upgrade-from-okd-scos-4.19-e2e-aws-ovn-upgrade/1937465859354136576/artifacts/e2e-aws-ovn-upgrade/gather-must-gather/artifacts/must-gather/inspect.local.3813551405126079732")
// 	files, err := fs.ReadDir(".")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	// clearly, this is broken.
// 	for i, file := range files {
// 		t.Logf("files[%d]: %v", i, file.Name())
// 	}

// 	file, err := fs.ReadFile("event-filter.html")
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	t.Logf("file: %v", string(file))

// 	file, err = fs.ReadFile("namespaces/kube-system/kube-system.yaml")
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	t.Logf("file: %v", string(file))

// 	file, err = fs.ReadFile("namespaces/kube-system/404")
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	t.Logf("file: %v", string(file))
// }

func TestGcsFs(t *testing.T) {
	fs, err := NewGcsFS("gs://test-platform-results/logs/periodic-ci-openshift-release-master-okd-scos-4.20-upgrade-from-okd-scos-4.19-e2e-aws-ovn-upgrade/1937465859354136576/artifacts/e2e-aws-ovn-upgrade/gather-must-gather/artifacts/must-gather/inspect.local.3813551405126079732")
	if err != nil {
		t.Fatal(err)
	}

	files, err := fs.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	// this works
	for i, file := range files {
		t.Logf("files[%d]: %v, isDir: %v", i, file.Name(), file.IsDir())
	}

	file, err := fs.ReadFile(path.Join(".", files[0].Name()))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("file: %v", string(file[:50]))

	_, err = fs.ReadDir(fs.Join("gs://test-platform-results/logs/periodic-ci-openshift-release-master-okd-scos-4.20-upgrade-from-okd-scos-4.19-e2e-aws-ovn-upgrade/1937465859354136576/artifacts/e2e-aws-ovn-upgrade/gather-must-gather/artifacts/must-gather/inspect.local.3813551405126079732", "namespaces"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetObjectPath(t *testing.T) {
	tests := []struct {
		name         string
		baseURL      string
		inputPath    string
		expectedPath string
	}{
		// Tests with empty objectPrefix
		{
			name:         "absolute GCS URL with same bucket, empty prefix",
			baseURL:      "gs://test-bucket",
			inputPath:    "gs://test-bucket/whatever/whatnot",
			expectedPath: "whatever/whatnot",
		},
		{
			name:         "absolute GCS URL with different bucket, empty prefix",
			baseURL:      "gs://test-bucket",
			inputPath:    "gs://other-bucket/whatever/whatnot",
			expectedPath: "whatever/whatnot",
		},
		{
			name:         "relative path with ./ prefix, empty prefix",
			baseURL:      "gs://test-bucket",
			inputPath:    "./whatever/foo",
			expectedPath: "whatever/foo",
		},
		{
			name:         "relative path without ./ prefix, empty prefix",
			baseURL:      "gs://test-bucket",
			inputPath:    "whatever/foo/another",
			expectedPath: "whatever/foo/another",
		},
		{
			name:         "absolute GCS URL with just bucket name, empty prefix",
			baseURL:      "gs://test-bucket",
			inputPath:    "gs://test-bucket",
			expectedPath: "",
		},

		// Tests with non-empty objectPrefix
		{
			name:         "absolute GCS URL with same bucket, with prefix",
			baseURL:      "gs://test-bucket/logs/some/path",
			inputPath:    "gs://test-bucket/whatever/whatnot",
			expectedPath: "whatever/whatnot",
		},
		{
			name:         "relative path with ./ prefix, with prefix",
			baseURL:      "gs://test-bucket/logs/some/path",
			inputPath:    "./whatever/foo",
			expectedPath: "logs/some/path/whatever/foo",
		},
		{
			name:         "relative path without ./ prefix, with prefix",
			baseURL:      "gs://test-bucket/logs/some/path",
			inputPath:    "whatever/foo/another",
			expectedPath: "logs/some/path/whatever/foo/another",
		},
		{
			name:         "single file relative path, with prefix",
			baseURL:      "gs://test-bucket/logs/some/path",
			inputPath:    "file.txt",
			expectedPath: "logs/some/path/file.txt",
		},
		{
			name:         "root relative path, with prefix",
			baseURL:      "gs://test-bucket/logs/some/path",
			inputPath:    ".",
			expectedPath: "logs/some/path",
		},
		{
			name:         "current dir relative path, with prefix",
			baseURL:      "gs://test-bucket/logs/some/path",
			inputPath:    "./",
			expectedPath: "logs/some/path",
		},

		// with "gs:/" also
		{
			name:         "absolute GCS URL with same bucket, with prefix",
			baseURL:      "gs://test-bucket/logs/some/path",
			inputPath:    "gs:/test-bucket/whatever/whatnot",
			expectedPath: "whatever/whatnot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &GcsFS{}

			trimmedBaseURL := strings.TrimPrefix(tt.baseURL, "gs://")
			bucketName := strings.Split(trimmedBaseURL, "/")[0]
			objectPrefix := strings.TrimPrefix(trimmedBaseURL, bucketName)
			objectPrefix = strings.TrimPrefix(objectPrefix, "/")

			fs.bucketName = bucketName
			fs.objectPrefix = objectPrefix

			// Test the getObjectPath method
			result := fs.getObjectPath(tt.inputPath)

			if result != tt.expectedPath {
				t.Errorf("getObjectPath(%q) = %q, want %q", tt.inputPath, result, tt.expectedPath)
			}
		})
	}
}
