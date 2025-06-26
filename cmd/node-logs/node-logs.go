package nodelogs

import (
	"fmt"
	"os"
	"strings"

	"github.com/gmeghnag/omc/vars"
	"github.com/spf13/cobra"
)

var NodeLogs = &cobra.Command{
	Use:   "node-logs",
	Short: "Display and filter node logs.",
	Run: func(cmd *cobra.Command, args []string) {
		output, err := nodeLogs(args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(output)
	},
}

func nodeLogs(args []string) (string, error) {
	if len(args) == 0 {
		var output strings.Builder
		output.WriteString("The following node service logs are available to be displayed:\n\n")
		files, err := os.ReadDir(vars.MustGatherRootPath + "/host_service_logs/masters/")
		if err != nil {
			return "", err
		}
		for _, f := range files {
			output.WriteString("- " + strings.TrimSuffix(f.Name(), "_service.log") + "\n")
		}
		output.WriteString("\nExecuting 'omc node-logs <SERVICE>' will display the logs.")
		return output.String(), nil
	}
	if len(args) > 1 {
		return "", fmt.Errorf("Expect zero argument, found: %d", len(args))
	}
	if len(args) == 1 {
		text, err := os.ReadFile(vars.MustGatherRootPath + "/host_service_logs/masters/" + args[0] + "_service.log")
		if err != nil {
			return "", fmt.Errorf("logs for service \"%s\" not found or readable", args[0])
		}
		return string(text), nil
	}
	return "", nil
}
