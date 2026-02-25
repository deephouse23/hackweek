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

var orgsCmd = &cobra.Command{
	Use:   "orgs",
	Short: "Manage organizations",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var orgsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List organizations you belong to",
	Example: "infisical orgs list",
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

		orgResponse, err := api.CallGetAllOrganizations(httpClient)
		if err != nil {
			util.HandleError(err, "Unable to fetch organizations")
		}

		outputFormat, _ := cmd.Flags().GetString("output")
		if outputFormat != "" {
			var outputStructure []map[string]any
			for _, org := range orgResponse.Organizations {
				outputStructure = append(outputStructure, map[string]any{
					"id":   org.ID,
					"name": org.Name,
				})
			}
			output, err := util.FormatOutput(outputFormat, outputStructure, nil)
			if err != nil {
				util.HandleError(err, "Unable to format output")
			}
			util.PrintStdout(output)
		} else {
			headers := []string{"ID", "NAME"}
			rows := [][]string{}
			for _, org := range orgResponse.Organizations {
				rows = append(rows, []string{org.ID, org.Name})
			}
			visualize.GenericTable(headers, rows)
		}

		Telemetry.CaptureEvent("cli-command:orgs list",
			posthog.NewProperties().Set("version", util.CLI_VERSION))
	},
}

func init() {
	util.AddOutputFlagsToCmd(orgsListCmd, "The output format for organizations")
	orgsCmd.AddCommand(orgsListCmd)
	RootCmd.AddCommand(orgsCmd)
}
