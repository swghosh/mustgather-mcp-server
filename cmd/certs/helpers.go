package certs

import (
	"fmt"
	"os"

	"github.com/gmeghnag/omc/cmd/helpers"
	"github.com/gmeghnag/omc/pkg/vfs"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type ResourcesItems struct {
	Kind       string                       `json:"kind"`
	ApiVersion string                       `json:"apiVersion"`
	Items      []*unstructured.Unstructured `json:"items"`
}

func GetSecrets(currentContextPath string, namespace string, resourceName string, allNamespacesFlag bool, out *[]*unstructured.Unstructured) {
	var namespaces []string
	if allNamespacesFlag == true {
		namespace = "all"
		_namespaces, _ := vfs.CurrentFS.ReadDir(vfs.CurrentFS.Join(currentContextPath, "namespaces"))
		for _, f := range _namespaces {
			namespaces = append(namespaces, f.Name())
		}
	} else {
		namespaces = append(namespaces, namespace)
	}

	for _, _namespace := range namespaces {
		var _Items ResourcesItems
		CurrentNamespacePath := vfs.CurrentFS.Join(currentContextPath, "namespaces", _namespace)
		_file, err := vfs.CurrentFS.ReadFile(vfs.CurrentFS.Join(CurrentNamespacePath, "core", "secrets.yaml"))
		if err != nil && !allNamespacesFlag {
			continue
		}
		if err := yaml.Unmarshal([]byte(_file), &_Items); err != nil {
			fmt.Fprintln(os.Stderr, "Error when trying to unmarshal file "+vfs.CurrentFS.Join(CurrentNamespacePath, "core", "secrets.yaml"))
			os.Exit(1)
		}

		for _, Secret := range _Items.Items {
			*out = append(*out, Secret)
		}
	}
}

func GetConfigMaps(currentContextPath string, namespace string, resourceName string, allNamespacesFlag bool, out *[]*unstructured.Unstructured) {
	var namespaces []string
	if allNamespacesFlag == true {
		namespace = "all"
		_namespaces, _ := vfs.CurrentFS.ReadDir(vfs.CurrentFS.Join(currentContextPath, "namespaces"))
		for _, f := range _namespaces {
			namespaces = append(namespaces, f.Name())
		}
	} else {
		namespaces = append(namespaces, namespace)
	}

	for _, _namespace := range namespaces {
		var _Items ResourcesItems
		CurrentNamespacePath := vfs.CurrentFS.Join(currentContextPath, "namespaces", _namespace)
		_file, err := vfs.CurrentFS.ReadFile(vfs.CurrentFS.Join(CurrentNamespacePath, "core", "configmaps.yaml"))
		if err != nil && !allNamespacesFlag {
			continue
		}
		if err := yaml.Unmarshal([]byte(_file), &_Items); err != nil {
			fmt.Fprintln(os.Stderr, "Error when trying to unmarshal file "+vfs.CurrentFS.Join(CurrentNamespacePath, "core", "configmaps.yaml"))
			os.Exit(1)
		}
		for _, ConfigMap := range _Items.Items {
			*out = append(*out, ConfigMap)
		}
	}
}

func GetCertificateSigningRequests(currentContextPath string, namespace string, resourceName string, allNamespacesFlag bool, out *[]unstructured.Unstructured) {

	certificatesigningrequestsFolderPath := vfs.CurrentFS.Join(currentContextPath, "cluster-scoped-resources", "certificates.k8s.io", "certificatesigningrequests")
	_certificatesigningrequests, _ := vfs.CurrentFS.ReadDir(certificatesigningrequestsFolderPath)

	for _, f := range _certificatesigningrequests {
		certificatesigningrequestYamlPath := vfs.CurrentFS.Join(certificatesigningrequestsFolderPath, f.Name())
		_file := helpers.ReadYaml(certificatesigningrequestYamlPath)
		CertificateSigningRequest := unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(_file), &CertificateSigningRequest); err != nil {
			fmt.Fprintln(os.Stderr, "Error when trying to unmarshal file: "+certificatesigningrequestYamlPath)
			os.Exit(1)
		}
		*out = append(*out, CertificateSigningRequest)
	}
}
