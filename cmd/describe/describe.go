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
package describe

import (
	"fmt"
	//"github.com/gmeghnag/omc/cmd/describe/apps"
	"os"
	"strings"

	"github.com/gmeghnag/omc/cmd/describe/core"

	"github.com/spf13/cobra"
)

// DescribeCmd represents the describe command
var DescribeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Show details of a specific resource or group of resources",
	Run: func(cmd *cobra.Command, args []string) {
		if err := describe(cmd, args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	},
}

func describe(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		cmd.Help()
		return nil
	}
	return fmt.Errorf("Invalid object type: %s", args[0])
}

func init() {
	if len(os.Args) > 2 && os.Args[1] == "describe" {
		if strings.Contains(os.Args[2], "/") {
			seg := strings.Split(os.Args[2], "/")
			resource, name := seg[0], seg[1]
			os.Args = append([]string{os.Args[0], "describe", resource, name}, os.Args[3:]...)
		}
	}
	DescribeCmd.AddCommand(
		//apps.Deployment,
		core.Node,
		core.Pod,
	)
}
