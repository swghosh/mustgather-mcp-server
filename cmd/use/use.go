/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package use

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gmeghnag/omc/cmd/helpers"
	"github.com/gmeghnag/omc/pkg/vfs"
	"github.com/gmeghnag/omc/types"
	"github.com/gmeghnag/omc/vars"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

var singleNamespaceInMustGather bool

func UseContext(fs vfs.Filesystem, path string, omcConfigFile string, idFlag string) error {
	if path != "" {
		_path, err := findMustGatherIn(fs, path)
		if err != nil {
			return err
		}
		vars.MustGatherRootPath = _path
		path = _path
	}

	// read json omcConfigFile
	file, _ := os.ReadFile(omcConfigFile)
	omcConfigJson := types.Config{}
	_ = json.Unmarshal([]byte(file), &omcConfigJson)

	var contexts []types.Context
	var NewContexts []types.Context
	contexts = omcConfigJson.Contexts
	var found bool
	var ctxId, configId string
	defaultProject := omcConfigJson.DefaultProject
	if defaultProject == "" {
		defaultProject = "default"
	}
	for _, c := range contexts {
		if c.Id == idFlag || c.Path == path {
			NewContexts = append(NewContexts, types.Context{Id: c.Id, Path: c.Path, Current: "*", Project: c.Project})
			configId = c.Id
			found = true
			vars.Namespace = c.Project
		} else {
			NewContexts = append(NewContexts, types.Context{Id: c.Id, Path: c.Path, Current: "", Project: c.Project})
		}
	}
	if !found {
		if idFlag != "" {
			NewContexts = append(NewContexts, types.Context{Id: idFlag, Path: path, Current: "*", Project: defaultProject})
		} else {
			ctxId = helpers.RandString(8)
			var namespaces []string

			_namespaces, _ := fs.ReadDir(fs.Join(path, "namespaces"))
			for _, f := range _namespaces {
				namespaces = append(namespaces, f.Name())
			}

			if len(namespaces) == 1 {
				NewContexts = append(NewContexts, types.Context{Id: ctxId, Path: path, Current: "*", Project: namespaces[0]})
				vars.Namespace = namespaces[0]
				singleNamespaceInMustGather = true
			} else {
				NewContexts = append(NewContexts, types.Context{Id: ctxId, Path: path, Current: "*", Project: defaultProject})
				vars.Namespace = defaultProject
			}
		}

	}

	if !found {
		if idFlag != "" {
			configId = idFlag
		} else {
			configId = ctxId
		}
	}
	config := types.Config{
		Id:             configId,
		Contexts:       NewContexts,
		DiffCmd:        omcConfigJson.DiffCmd,
		DefaultProject: omcConfigJson.DefaultProject,
		UseLocalCRDs:   omcConfigJson.UseLocalCRDs,
	}
	file, _ = json.MarshalIndent(config, "", " ")
	_ = os.WriteFile(omcConfigFile, file, 0644)
	return nil
}

func findMustGatherIn(fs vfs.Filesystem, path string) (string, error) {
	if IsGCSPath(path) || IsRemoteFile(path) {
		return path, nil
	}

	files, err := fs.ReadDir(path)
	if err != nil {
		return "", err
	}

	var resourcesFolderFound bool
	var timeStampFound bool
	var subDir string

	for _, file := range files {
		if file.Name() == "cluster-scoped-resources" {
			resourcesFolderFound = true
		}
		if file.Name() == "timestamp" {
			timeStampFound = true
		}
		if file.IsDir() {
			subDir = file.Name()
		}
	}

	if resourcesFolderFound && timeStampFound {
		return path, nil
	}

	if len(files) == 1 && subDir != "" {
		return findMustGatherIn(fs, fs.Join(path, subDir))
	}

	return path, nil
}

func MustGatherInfo() {
	fmt.Printf("Must-Gather    : %s\n", vars.MustGatherRootPath)
	if singleNamespaceInMustGather {
		fmt.Printf("Project        : %s (single project)\n", vars.Namespace)
	} else {
		fmt.Printf("Project        : %s\n", vars.Namespace)
	}
	InfrastructureFilePathExists, _ := helpers.Exists(vfs.OS.Join(vars.MustGatherRootPath, "cluster-scoped-resources/config.openshift.io/infrastructures.yaml"))
	if InfrastructureFilePathExists {
		_file, _ := vfs.OS.ReadFile(vfs.OS.Join(vars.MustGatherRootPath, "cluster-scoped-resources/config.openshift.io/infrastructures.yaml"))
		infrastructureList := configv1.InfrastructureList{}
		if err := yaml.Unmarshal([]byte(_file), &infrastructureList); err != nil {
			fmt.Println("Error when trying to unmarshal file: " + vfs.OS.Join(vars.MustGatherRootPath, "/cluster-scoped-resources/config.openshift.io/infrastructures.yaml"))
			os.Exit(1)
		} else {
			fmt.Printf("ApiServerURL   : %s\n", infrastructureList.Items[0].Status.APIServerURL)
			fmt.Printf("Platform       : %s\n", infrastructureList.Items[0].Status.PlatformStatus.Type)
		}
	}
	clusterversionFilePathExists, _ := helpers.Exists(vfs.OS.Join(vars.MustGatherRootPath, "cluster-scoped-resources/config.openshift.io/clusterversions/version.yaml"))
	if clusterversionFilePathExists {
		_file, _ := vfs.OS.ReadFile(vfs.OS.Join(vars.MustGatherRootPath, "cluster-scoped-resources/config.openshift.io/clusterversions/version.yaml"))
		ClusterVersion := configv1.ClusterVersion{}
		if err := yaml.Unmarshal([]byte(_file), &ClusterVersion); err != nil {
			fmt.Println("Error when trying to unmarshal file: " + vfs.OS.Join(vars.MustGatherRootPath, "/cluster-scoped-resources/config.openshift.io/clusterversions/version.yaml"))
			os.Exit(1)
		} else {
			clusterversion := ""
			versionHistory := ClusterVersion.Status.History
			for _, version := range versionHistory {
				if version.State == "Completed" {
					clusterversion = version.Version
					break
				}
			}

			fmt.Printf("ClusterID      : %s\n", ClusterVersion.Spec.ClusterID)
			fmt.Printf("ClusterVersion : %s\n", clusterversion)
		}
	}
	mustGatherParentPath := ""
	if IsRemoteFile(vars.MustGatherRootPath) {
		u, err := url.Parse(vars.MustGatherRootPath)
		if err == nil {
			u.Path = path.Dir(u.Path)
			mustGatherParentPath = u.String()
		} else {
			mustGatherParentPath = vars.MustGatherRootPath
		}
	} else {
		mustGatherParentPath = filepath.Dir(vars.MustGatherRootPath)
	}

	clientVersion := extractClientVersion(vfs.OS.Join(mustGatherParentPath, "must-gather.logs"))
	if clientVersion != "" {
		fmt.Printf("ClientVersion  : %s\n", clientVersion)
	}
	parts := strings.Split(vars.MustGatherRootPath, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if strings.Contains(lastPart, "-sha256") {
			mustGatherImage := strings.Split(lastPart, "-sha256")[0]
			fmt.Printf("Image          : %s\n", mustGatherImage)
		}
	}

}

// useCmd represents the use command
var UseCmd = &cobra.Command{
	Use:   "use",
	Short: "Select the must-gather to use",
	Long: `
	Select the must-gather to use.
	If the must-gather does not exists it will be added as default to the managed must-gathers.
	Use the command 'omc get mg' to see them all.`,
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		idFlag, _ := cmd.Flags().GetString("id")
		path := ""
		isCompressedFile := false
		fileType := ""
		fs := vfs.OS

		if len(args) == 0 && idFlag == "" {
			MustGatherInfo()
			os.Exit(0)
		}
		if len(args) > 1 {
			fmt.Fprintln(os.Stderr, "Expect one argument, found: ", len(args))
			os.Exit(1)
		}
		if len(args) == 1 {
			path = args[0]
			if IsGCSPath(path) {
				fs, err = vfs.NewGcsFS(path)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error creating GCS filesystem: ", err)
					os.Exit(1)
				}
			} else if IsRemoteFile(path) {
				fs = vfs.NewHttpFS(path)
			} else {
				if strings.HasSuffix(path, "/") {
					path = strings.TrimRight(path, "/")
				}
				if strings.HasSuffix(path, "\\") {
					path = strings.TrimRight(path, "\\")
				}
				path, _ = filepath.Abs(path)

				isDir, _ := helpers.IsDirectory(path)
				if !isDir {
					isCompressedFile, fileType, _ = IsCompressedFile(path)
					if !isCompressedFile {
						fmt.Fprintln(os.Stderr, "Error: "+path+" is not a directory nor a compressed file.")
						os.Exit(1)
					}
				}
			}
		}

		if isCompressedFile {
			outputpath := filepath.Dir(path)
			rootfile, err := DecompressFile(path, outputpath, fileType)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: decompressing "+path+" in "+outputpath+": "+err.Error())
				os.Exit(1)
			}
			path = rootfile
		}

		err = UseContext(fs, path, viper.ConfigFileUsed(), idFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		MustGatherInfo()
	},
}

func IsGCSPath(path string) bool {
	return strings.HasPrefix(path, "gs://")
}

func init() {
	UseCmd.Flags().StringVarP(&vars.Id, "id", "i", "", "Id string for the must-gather to use. If two must-gather has the same id the first one will be used.")
}
