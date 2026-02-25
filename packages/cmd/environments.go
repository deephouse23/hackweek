/*
Copyright (c) 2023 Infisical Inc.
*/
package cmd

import (
	"github.com/Infisical/infisical-merge/packages/api"
	"github.com/Infisical/infisical-merge/packages/util"
	"github.com/Infisical/infisical-merge/packages/visualize"
	"github.com/posthog/posthog-go"
	"github.com/spf13/cobra"
)

var environmentsCmd = &cobra.Command{
	Use:   "environments",
	Short: "Manage project environments",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var environmentsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List accessible environments for a project",
	Example: "infisical environments list\ninfisical environments list --projectId=ID",
	Args:    cobra.NoArgs,
	PreRun: func(cmd *cobra.Command, args []string) {
		util.RequireLogin()
	},
	Run: func(cmd *cobra.Command, args []string) {
		loggedInUserDetails, err := util.GetCurrentLoggedInUserDetails(true)
		if err != nil {
			util.HandleError(err, "Unable to authenticate")
		}

		if loggedInUserDetails.LoginExpired {
			loggedInUserDetails = util.EstablishUserLoginSession()
		}

		httpClient, err := util.GetRestyClientWithCustomHeaders()
		if err != nil {
			util.HandleError(err, "Unable to create HTTP client")
		}
		httpClient.SetAuthToken(loggedInUserDetails.UserCredentials.JTWToken)

		projectId, _ := cmd.Flags().GetString("projectId")
		if projectId == "" {
			workspaceFile, err := util.GetWorkSpaceFromFile()
			if err != nil {
				util.PrintErrorMessageAndExit("No project linked. Run 'infisical init' or pass --projectId")
			}
			projectId = workspaceFile.WorkspaceId
		}

		envResponse, err := api.CallGetAccessibleEnvironments(httpClient, api.GetAccessibleEnvironmentsRequest{
			WorkspaceId: projectId,
		})
		if err != nil {
			util.HandleError(err, "Unable to fetch environments")
		}

		outputFormat, _ := cmd.Flags().GetString("output")
		if outputFormat != "" {
			var outputStructure []map[string]any
			for _, env := range envResponse.AccessibleEnvironments {
				outputStructure = append(outputStructure, map[string]any{
					"name":          env.Name,
					"slug":          env.Slug,
					"isWriteDenied": env.IsWriteDenied,
				})
			}
			output, err := util.FormatOutput(outputFormat, outputStructure, nil)
			if err != nil {
				util.HandleError(err, "Unable to format output")
			}
			util.PrintStdout(output)
		} else {
			headers := []string{"NAME", "SLUG", "WRITE ACCESS"}
			rows := [][]string{}
			for _, env := range envResponse.AccessibleEnvironments {
				writeAccess := "yes"
				if env.IsWriteDenied {
					writeAccess = "no"
				}
				rows = append(rows, []string{env.Name, env.Slug, writeAccess})
			}
			visualize.GenericTable(headers, rows)
		}

		Telemetry.CaptureEvent("cli-command:environments list",
			posthog.NewProperties().Set("version", util.CLI_VERSION))
	},
}

func init() {
	environmentsListCmd.Flags().String("projectId", "", "The project ID (defaults to current linked project)")
	util.AddOutputFlagsToCmd(environmentsListCmd, "The output format for environments")
	environmentsCmd.AddCommand(environmentsListCmd)
	RootCmd.AddCommand(environmentsCmd)
}
