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
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gmeghnag/omc/types"
	"github.com/gmeghnag/omc/vars"
	"github.com/spf13/cobra"
)

var test = false
var ConfigCmd = &cobra.Command{
	Use: "config",
	Run: func(cmd *cobra.Command, args []string) {
		if err := SetConfig(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	},
}

func init() {
	ConfigCmd.PersistentFlags().BoolVarP(&vars.UseLocalCRDs, "use-local-crds", "", false, "If set to true, omc will search for valid CRDs also in ~/.omc/customresourcedefinitions")
	ConfigCmd.PersistentFlags().StringVarP(&vars.DiffCmd, "diff-command", "", "", "Set the binary tool to use to execute \"omc mc diff <machineConfig1> <machineConfig2>\"")
	ConfigCmd.PersistentFlags().StringVarP(&vars.DefaultProject, "default-project", "", "", "Set the default context project \"omc config --default-project=<NS>\"")

}

func SetConfig() error {
	home, _ := os.UserHomeDir()
	file, err := ioutil.ReadFile(home + "/.omc/omc.json")
	if err != nil {
		return err
	}
	omcConfigJson := types.Config{}
	_ = json.Unmarshal([]byte(file), &omcConfigJson)
	omcConfigJson.UseLocalCRDs = vars.UseLocalCRDs
	omcConfigJson.DiffCmd = vars.DiffCmd
	omcConfigJson.DefaultProject = vars.DefaultProject
	file, err = json.MarshalIndent(omcConfigJson, "", " ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(home+"/.omc/omc.json", file, 0644)
}
