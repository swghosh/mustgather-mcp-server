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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gmeghnag/omc/cmd/helpers"
	"github.com/gmeghnag/omc/types"
	"github.com/gmeghnag/omc/vars"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

var singleNamespaceInMustGather bool

func use(path, idFlag, omcConfigFile string) (string, error) {
	if path == "" && idFlag == "" {
		return MustGatherInfo()
	}

	var err error
	fileType := ""
	isCompressedFile := false
	if path != "" {
		if IsRemoteFile(path) {
			path, err = DownloadFile(path)
			if err != nil {
				return "", err
			}
		} else {
			if strings.HasSuffix(path, "/") {
				path = strings.TrimRight(path, "/")
			}
			if strings.HasSuffix(path, "\\") {
				path = strings.TrimRight(path, "\\")
			}
			path, _ = filepath.Abs(path)
		}

		isDir, _ := helpers.IsDirectory(path)
		if !isDir {
			isCompressedFile, fileType, _ = IsCompressedFile(path)
			if !isCompressedFile {
				return "", fmt.Errorf("Error: " + path + " is not a directory not a compressed file.")
			}
		}
	}

	if isCompressedFile {
		outputpath := filepath.Dir(path)
		rootfile, err := DecompressFile(path, outputpath, fileType)
		if err != nil {
			return "", fmt.Errorf("Error: decompressing " + path + " in " + outputpath + ": " + err.Error())
		}
		path = rootfile
	}

	err = useContext(path, omcConfigFile, idFlag)
	if err != nil {
		return "", err
	}
	return MustGatherInfo()
}

func useContext(path string, omcConfigFile string, idFlag string) error {
	if path != "" {
		_path, err := findMustGatherIn(path)
		if err != nil {
			return err
		}
		l := strings.Split(_path, "/")
		path = strings.Join(l[0:(len(l)-1)], "/")
		path = strings.TrimSuffix(path, "/")
		vars.MustGatherRootPath = path
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
			_namespaces, _ := os.ReadDir(path + "/namespaces/")
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

func findMustGatherIn(path string) (string, error) {
	numDirs := 0
	dirName := ""
	retPath := strings.TrimSuffix(path, "/")
	var retErr error
	timeStampFound := false
	resourcesFolderFound := false
	files, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	for _, file := range files {
		if file.IsDir() {
			dirName = file.Name()
			numDirs = numDirs + 1
			if file.Name() == "namespaces" || file.Name() == "cluster-scoped-resources" {
				resourcesFolderFound = true
			}
		}
		if !file.IsDir() && file.Name() == "timestamp" {
			timeStampFound = true
		}
	}
	if numDirs == 1 && !timeStampFound && !resourcesFolderFound {
		return findMustGatherIn(path + "/" + dirName)
	}
	if resourcesFolderFound {
		return retPath + "/", retErr
	}
	if timeStampFound && (numDirs > 1 || numDirs == 0) {
		return path, fmt.Errorf("expected one directory in path: \"%s\", found: %s", path, strconv.Itoa(numDirs))
	}
	if !timeStampFound && !resourcesFolderFound {
		// Case: "path" is an empty directory
		return "", fmt.Errorf("wrong must-gather file composition for %v", path)
	}
	return findMustGatherIn(path + "/" + dirName)
}

func MustGatherInfo() (string, error) {
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Must-Gather    : %s\n", vars.MustGatherRootPath))
	if singleNamespaceInMustGather {
		output.WriteString(fmt.Sprintf("Project        : %s (single project)\n", vars.Namespace))
	} else {
		output.WriteString(fmt.Sprintf("Project        : %s\n", vars.Namespace))
	}
	InfrastrctureFilePathExists, err := helpers.Exists(vars.MustGatherRootPath + "/cluster-scoped-resources/config.openshift.io/infrastructures.yaml")
	if err != nil {
		return "", err
	}
	if InfrastrctureFilePathExists {
		_file, err := os.ReadFile(vars.MustGatherRootPath + "/cluster-scoped-resources/config.openshift.io/infrastructures.yaml")
		if err != nil {
			return "", err
		}
		infrastructureList := configv1.InfrastructureList{}
		if err := yaml.Unmarshal([]byte(_file), &infrastructureList); err != nil {
			return "", fmt.Errorf("Error when trying to unmarshal file: " + vars.MustGatherRootPath + "/cluster-scoped-resources/config.openshift.io/infrastructures.yaml")
		} else {
			output.WriteString(fmt.Sprintf("ApiServerURL   : %s\n", infrastructureList.Items[0].Status.APIServerURL))
			output.WriteString(fmt.Sprintf("Platform       : %s\n", infrastructureList.Items[0].Status.PlatformStatus.Type))
		}
	}
	clusterversionFilePathExists, err := helpers.Exists(vars.MustGatherRootPath + "/cluster-scoped-resources/config.openshift.io/clusterversions/version.yaml")
	if err != nil {
		return "", err
	}
	if clusterversionFilePathExists {
		_file, err := os.ReadFile(vars.MustGatherRootPath + "/cluster-scoped-resources/config.openshift.io/clusterversions/version.yaml")
		if err != nil {
			return "", err
		}
		ClusterVersion := configv1.ClusterVersion{}
		if err := yaml.Unmarshal([]byte(_file), &ClusterVersion); err != nil {
			return "", fmt.Errorf("Error when trying to unmarshal file: " + vars.MustGatherRootPath + "/cluster-scoped-resources/config.openshift.io/clusterversions/version.yaml")
		} else {
			clusterversion := ""
			versionHistory := ClusterVersion.Status.History
			for _, version := range versionHistory {
				if version.State == "Completed" {
					clusterversion = version.Version
					break
				}
			}

			output.WriteString(fmt.Sprintf("ClusterID      : %s\n", ClusterVersion.Spec.ClusterID))
			output.WriteString(fmt.Sprintf("ClusterVersion : %s\n", clusterversion))
		}
	}
	mustGatherSplitPath := strings.Split(vars.MustGatherRootPath, "/")
	mustGatherParentPath := strings.Join(mustGatherSplitPath[0:(len(mustGatherSplitPath)-1)], "/")
	clientVersion := extractClientVersion(mustGatherParentPath + "/must-gather.logs")
	if clientVersion != "" {
		output.WriteString(fmt.Sprintf("ClientVersion  : %s\n", clientVersion))
	}
	parts := strings.Split(vars.MustGatherRootPath, "/")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if strings.Contains(lastPart, "-sha256") {
			mustGatherImage := strings.Split(lastPart, "-sha256")[0]
			output.WriteString(fmt.Sprintf("Image          : %s\n", mustGatherImage))
		}
	}
	return output.String(), nil
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
		idFlag, _ := cmd.Flags().GetString("id")
		var path string
		if len(args) > 1 {
			fmt.Fprintln(os.Stderr, "Expect one argument, found: ", len(args))
			os.Exit(1)
		}
		if len(args) == 1 {
			path = args[0]
		}
		output, err := use(path, idFlag, viper.ConfigFileUsed())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(output)
	},
}

func init() {
	UseCmd.Flags().StringVarP(&vars.Id, "id", "i", "", "Id string for the must-gather to use. If two must-gather has the same id the first one will be used.")
}
