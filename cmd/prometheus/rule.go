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
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gmeghnag/omc/cmd/helpers"
	"github.com/gmeghnag/omc/vars"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func alertRules(resourcesNames []string, outputFlag, groupsNames, rulesStates string) (string, error) {
	monitoringExist, err := helpers.Exists(vars.MustGatherRootPath + "/monitoring")
	if err != nil {
		return "", err
	}
	if !monitoringExist {
		return "", fmt.Errorf("Path '" + vars.MustGatherRootPath + "/monitoring' does not exist.")
	}
	alertsFilePath := vars.MustGatherRootPath + "/monitoring/alerts.json"
	alertsFilePathExist, err := helpers.Exists(alertsFilePath)
	if err != nil {
		return "", err
	}
	if !alertsFilePathExist {
		alertsFilePath = vars.MustGatherRootPath + "/monitoring/prometheus/rules.json"
		alertsFilePathExist, err = helpers.Exists(alertsFilePath)
		if err != nil {
			return "", err
		}
		if !alertsFilePathExist {
			return "", fmt.Errorf("Prometheus rules not found in must-gather.")
		}
	}
	return GetAlertRules(resourcesNames, outputFlag, groupsNames, rulesStates, alertsFilePath)
}

func GetAlertRules(resourcesNames []string, outputFlag string, groupsNames string, rulesStates string, alertsFilePath string) (string, error) {
	_headers := []string{"group", "rule", "severity", "state", "age", "alerts", "active since"}
	var data [][]string
	var filteredRules []Rule
	var filteredRulesList FilteredRulesList
	var _Alerts alerts
	_file, _ := ioutil.ReadFile(alertsFilePath)
	if err := yaml.Unmarshal([]byte(_file), &_Alerts); err != nil {
		return "", fmt.Errorf("Error when trying to unmarshal file " + alertsFilePath)
	}
	searchingGroups := []string{}
	if groupsNames != "" {
		searchingGroups = strings.Split(groupsNames, ",")
	}
	searchingStates := []string{}
	if rulesStates != "" {
		searchingStates = strings.Split(rulesStates, ",")
	}

	for _, group := range _Alerts.Data.Groups {
		if len(searchingGroups) != 0 && !helpers.StringInSlice(group.Name, searchingGroups) {
			continue
		}

		for _, rule := range group.Rules {
			ruleType := fmt.Sprint(rule["type"])
			if ruleType == "recording" {
				continue
			}
			ruleName := fmt.Sprint(rule["name"])
			if len(resourcesNames) != 0 && !helpers.StringInSlice(ruleName, resourcesNames) {
				continue
			}
			ruleLables := rule["labels"].(map[string]interface{})
			ruleSeverity := fmt.Sprint(ruleLables["severity"])
			ruleState := fmt.Sprint(rule["state"])
			if len(searchingStates) != 0 && !helpers.StringInSlice(ruleState, searchingStates) {
				continue
			}

			if outputFlag == "yaml" || outputFlag == "json" {
				filteredRules = append(filteredRules, rule)
				continue
			}

			activeSince := "----"
			// I didn't found any other solution than this (Marshal and Unmarshal) to transform alerts interface{} to []PromAlert{} :/
			alerts := rule["alerts"]
			alertsList := []PromAlert{}
			b, err := json.Marshal(alerts)
			if err != nil {
				return "", err
			}
			json.Unmarshal(b, &alertsList)
			numAlerts := strconv.Itoa(len(alertsList))
			if len(alertsList) != 0 {
				if len(alertsList) > 1 {
					firstOccur := alertsList[0].ActiveAt
					for i := range alertsList[1:] {
						alertBefore := alertsList[i].ActiveAt
						if alertsList[i+1].ActiveAt.Before(*alertBefore) {
							firstOccur = alertsList[i+1].ActiveAt
						}
					}
					activeSince = firstOccur.Format(time.RFC822)
				} else {
					alert := alertsList[0]
					activeSince = alert.ActiveAt.Format(time.RFC822)
				}
			}
			ruleLastEvaluation := fmt.Sprint(rule["lastEvaluation"])
			ruleLastEvaluationTime, _ := time.Parse(time.RFC3339Nano, ruleLastEvaluation)
			ResourceFile, _ := os.Stat(alertsFilePath)
			t2 := ResourceFile.ModTime()
			diffTime := t2.Sub(ruleLastEvaluationTime).String()
			d, _ := time.ParseDuration(diffTime)
			lastEval := helpers.FormatDiffTime(d)
			_list := []string{group.Name, ruleName, ruleSeverity, ruleState, lastEval, numAlerts, activeSince}
			showGroup := false
			if outputFlag == "wide" {
				showGroup = true
			}
			data = helpers.GetData(data, showGroup, false, "", outputFlag, 7, _list)
		}
	}

	var headers []string
	var output strings.Builder
	if outputFlag == "" {
		headers = _headers[1:]
		if len(data) == 0 {
			output.WriteString("No resources found.")
		} else {
			helpers.RenderTable(&output, headers, data)
		}
	}
	if outputFlag == "wide" {
		headers = _headers[0:]
		if len(data) == 0 {
			output.WriteString("No resources found.")
		} else {
			helpers.RenderTable(&output, headers, data)
		}
	}
	if outputFlag == "yaml" {
		filteredRulesList.Data = filteredRules
		y, _ := yaml.Marshal(filteredRulesList)
		output.Write(y)
	}
	if outputFlag == "json" {
		filteredRulesList.Data = filteredRules
		j, _ := json.Marshal(filteredRulesList)
		output.Write(j)
	}
	return output.String(), nil
}

var RuleSubCmd = &cobra.Command{
	Use:     "alertrule",
	Aliases: []string{"rule", "rules", "alertrules"},
	Short:   "Retrieve the alerting rules (and their status) configured in Prometheus.",
	Run: func(cmd *cobra.Command, args []string) {
		output, err := alertRules(args, vars.OutputStringVar, GroupName, RuleState)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(output)
	},
}

func init() {
	RuleSubCmd.Flags().StringVarP(&GroupName, "group", "g", "", "Filter the AlertRules by AlertGroup/s (comma separated).")
	RuleSubCmd.Flags().StringVarP(&RuleState, "state", "s", "", "Filter the AlertRules by state.")
	RuleSubCmd.Flags().StringVarP(&vars.OutputStringVar, "output", "o", "", "Output format. One of: json|yaml")
}
