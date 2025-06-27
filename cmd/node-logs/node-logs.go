package nodelogs

import (
	"fmt"
	"os"
	"strings"

	"github.com/gmeghnag/omc/pkg/vfs"
	"github.com/gmeghnag/omc/vars"
	"github.com/spf13/cobra"
)

var NodeLogs = &cobra.Command{
	Use:   "node-logs",
	Short: "Display and filter node logs.",
	Run: func(cmd *cobra.Command, args []string) {
		logsPath := vfs.CurrentFS.Join(vars.MustGatherRootPath, "host_service_logs", "masters")
		if len(args) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "The following node service logs are available to be displayed:")
			fmt.Fprintln(cmd.OutOrStdout(), "")
			files, _ := vfs.CurrentFS.ReadDir(logsPath)
			for _, f := range files {
				fmt.Fprintln(cmd.OutOrStdout(), "-", strings.TrimSuffix(f.Name(), "_service.log"))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nExecuting 'omc node-logs <SERVICE>' will display the logs.")
		}
		if len(args) > 1 {
			fmt.Fprintln(cmd.ErrOrStderr(), "Expect zero arguemnt, found: ", len(args))
			os.Exit(1)
		}
		if len(args) == 1 {
			logFile := vfs.CurrentFS.Join(logsPath, args[0]+"_service.log")
			text, err := vfs.CurrentFS.ReadFile(logFile)
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "logs for service \""+args[0]+"\" not found or readable.")
				os.Exit(1)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(text))
		}
	},
}
