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
package prometheus

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gmeghnag/omc/cmd/helpers"
	"github.com/gmeghnag/omc/pkg/vfs"
	"github.com/gmeghnag/omc/vars"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func GetAlertGroups(cmd *cobra.Command, resourcesNames []string, outputFlag string, groupFile string, alertsFilePath string) {
	_headers := []string{"group", "filename", "age"}
	var data [][]string
	var filteredGroups []RuleGroup
	var _Alerts alerts
	_file, _ := vfs.CurrentFS.ReadFile(alertsFilePath)
	if err := yaml.Unmarshal([]byte(_file), &_Alerts); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "Error when trying to unmarshal file "+alertsFilePath)
		os.Exit(1)
	}

	for _, group := range _Alerts.Data.Groups {
		filename := group.File[strings.LastIndex(group.File, "/")+1:]
		if len(resourcesNames) != 0 && !helpers.StringInSlice(group.Name, resourcesNames) {
			continue
		}

		if groupFile != "" && filename != groupFile {
			continue
		}

		if outputFlag == "yaml" || outputFlag == "json" {
			filteredGroups = append(filteredGroups, group)
			continue
		}

		//fmt.Println(al.Name, filename)
		ResourceFile, _ := vfs.CurrentFS.Stat(alertsFilePath)
		t2 := ResourceFile.ModTime()
		diffTime := t2.Sub(group.LastEvaluation).String()
		d, _ := time.ParseDuration(diffTime)
		lastEval := helpers.FormatDiffTime(d)
		_list := []string{group.Name, filename, lastEval}
		data = helpers.GetData(data, true, false, "", "", 3, _list)
	}

	var headers []string
	if outputFlag == "" || outputFlag == "wide" {
		headers = _headers[0:3]
		if len(data) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No alertgroups found.")
		} else {
			helpers.PrintTable(cmd, headers, data)
		}
	}
	if outputFlag == "yaml" {
		_Alerts.Data.Groups = filteredGroups
		y, _ := yaml.Marshal(_Alerts)
		fmt.Fprintln(cmd.OutOrStdout(), string(y))
	}
	if outputFlag == "json" {
		_Alerts.Data.Groups = filteredGroups
		j, _ := json.Marshal(_Alerts)
		fmt.Fprintln(cmd.OutOrStdout(), string(j))
	}

}

var GroupSubCmd = &cobra.Command{
	Use:     "alertgroup",
	Aliases: []string{"alertgroups", "group", "groups"},
	Short:   "Retrieve the alerting rules' groups configured in Prometheus.",
	Run: func(cmd *cobra.Command, args []string) {
		resourcesNames := args
		monitoringPath := vfs.CurrentFS.Join(vars.MustGatherRootPath, "monitoring")
		monitoringExist, _ := helpers.Exists(monitoringPath)
		if !monitoringExist {
			fmt.Fprintln(cmd.ErrOrStderr(), "Path '"+monitoringPath+"' does not exist.")
			os.Exit(1)
		}
		alertsFilePath := vfs.CurrentFS.Join(vars.MustGatherRootPath, "monitoring", "alerts.json")
		alertsFilePathExist, _ := helpers.Exists(alertsFilePath)
		if !alertsFilePathExist {
			alertsFilePath = vfs.CurrentFS.Join(vars.MustGatherRootPath, "monitoring", "prometheus", "rules.json")
			alertsFilePathExist, _ := helpers.Exists(alertsFilePath)
			if !alertsFilePathExist {
				fmt.Fprintln(cmd.ErrOrStderr(), "Prometheus rules not found in must-gather.")
				os.Exit(1)
			}
		}
		GetAlertGroups(cmd, resourcesNames, vars.OutputStringVar, GroupFilename, alertsFilePath)
	},
}

func init() {
	GroupSubCmd.Flags().StringVarP(&GroupFilename, "filename", "f", "", "Filter the AlertGroup by filename.")
	GroupSubCmd.Flags().StringVarP(&vars.OutputStringVar, "output", "o", "", "Output format. One of: json|yaml")
}
