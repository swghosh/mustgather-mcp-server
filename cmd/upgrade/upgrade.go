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
package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/coreos/go-semver/semver"
	"github.com/gmeghnag/omc/vars"
	"github.com/spf13/cobra"
)

var DesiredVersion string

const omcDarwinFile = "omc_Darwin"

func upgrade(repoName string, desiredVersion string) (string, error) {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	omcExecutablePath := filepath.Dir(ex) + "/omc"
	operatingSystem := runtime.GOOS
	if desiredVersion == "" {
		return checkReleases(repoName)
	}
	if desiredVersion != "latest" && string(desiredVersion[0]) != "v" {
		return "", fmt.Errorf("error: --to must be a semantic version (e.g. v4.0.5): No Major.Minor.Patch elements found")
	}
	if desiredVersion != "latest" {
		desiredReleaseVer := semver.New(desiredVersion[1:])
		if vars.OMCVersionTag == "" {
			vars.OMCVersionTag = "v2.0.1"
		}
		currentVer := semver.New(vars.OMCVersionTag[1:])
		if desiredReleaseVer.LessThan(*currentVer) {
			return "", fmt.Errorf("error: The update " + desiredVersion + " is not one of the available updates (check them by running \"omc upgrade\")")
		}
	}
	switch operatingSystem {
	case "windows":
		return "This command is not available for windows.\nOpen an issue on the GitHub repo https://github.com/gmeghnag/omc if you want it impemented.", nil
	case "darwin":
		arch := runtime.GOARCH
		omcBinFile := omcDarwinFile + "_" + arch
		omcUrl := "https://github.com/" + repoName + "/releases/download/" + desiredVersion + "/" + omcBinFile
		if desiredVersion == "latest" {
			omcUrl = "https://github.com/" + repoName + "/releases/" + desiredVersion + "/download/" + omcBinFile
		}
		err = updateOmcExecutable(omcExecutablePath, omcUrl, desiredVersion)
		if err != nil {
			return "", err
		}
	case "linux":
		omcUrl := "https://github.com/" + repoName + "/releases/download/" + desiredVersion + "/omc_Linux_x86_64"
		if desiredVersion == "latest" {
			omcUrl = "https://github.com/" + repoName + "/releases/" + desiredVersion + "/download/omc_Linux_x86_64"
		}
		err = updateOmcExecutable(omcExecutablePath, omcUrl, desiredVersion)
		if err != nil {
			return "", err
		}
	default:
		return "This command is not available for the OS you are using.\nOpen an issue on the GitHub repo https://github.com/gmeghnag/omc if you want it impemented.", nil
	}
	return "", nil
}

// etcdCmd represents the etcd command
var Upgrade = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade omc.",
	Run: func(cmd *cobra.Command, args []string) {
		output, err := upgrade("gmeghnag/omc", DesiredVersion)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(output)
	},
}

func init() {
	Upgrade.Flags().StringVarP(&DesiredVersion, "to", "", "", "Specify the version to upgrade to. The version must be on the list of available updates.")
}
