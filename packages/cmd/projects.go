/*
Copyright (c) 2023 Infisical Inc.
*/
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/Infisical/infisical-merge/packages/api"
	"github.com/Infisical/infisical-merge/packages/models"
	"github.com/Infisical/infisical-merge/packages/util"
	"github.com/Infisical/infisical-merge/packages/visualize"
	"github.com/posthog/posthog-go"
	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage projects and workspaces",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var projectsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all projects you have access to",
	Example: "infisical projects list\ninfisical projects list --org-id=ORG_ID",
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

		orgId, _ := cmd.Flags().GetString("org-id")

		// If org-id is provided, we need to select that org first to scope the workspace list
		if orgId != "" {
			tokenResponse, err := api.CallSelectOrganization(httpClient, api.SelectOrganizationRequest{OrganizationId: orgId})
			if err != nil {
				util.HandleError(err, "Unable to select organization")
			}
			httpClient.SetAuthToken(tokenResponse.Token)
		}

		workspaceResponse, err := api.CallGetAllWorkSpacesUserBelongsTo(httpClient)
		if err != nil {
			util.HandleError(err, "Unable to fetch projects")
		}

		workspaces := workspaceResponse.Workspaces

		// Filter by org-id if provided
		if orgId != "" {
			var filtered []struct {
				ID             string `json:"_id"`
				Name           string `json:"name"`
				Plan           string `json:"plan,omitempty"`
				V              int    `json:"__v"`
				OrganizationId string `json:"orgId"`
			}
			for _, ws := range workspaces {
				if ws.OrganizationId == orgId {
					filtered = append(filtered, ws)
				}
			}
			workspaces = filtered
		}

		outputFormat, _ := cmd.Flags().GetString("output")
		if outputFormat != "" {
			var outputStructure []map[string]any
			for _, ws := range workspaces {
				outputStructure = append(outputStructure, map[string]any{
					"id":    ws.ID,
					"name":  ws.Name,
					"orgId": ws.OrganizationId,
				})
			}
			output, err := util.FormatOutput(outputFormat, outputStructure, nil)
			if err != nil {
				util.HandleError(err, "Unable to format output")
			}
			util.PrintStdout(output)
		} else {
			headers := []string{"ID", "NAME", "ORG ID"}
			rows := [][]string{}
			for _, ws := range workspaces {
				rows = append(rows, []string{ws.ID, ws.Name, ws.OrganizationId})
			}
			visualize.GenericTable(headers, rows)
		}

		Telemetry.CaptureEvent("cli-command:projects list",
			posthog.NewProperties().Set("version", util.CLI_VERSION))
	},
}

var projectsSwitchCmd = &cobra.Command{
	Use:     "switch [project-id]",
	Short:   "Switch the active project for this directory",
	Example: "infisical projects switch <project-id>",
	Args:    cobra.ExactArgs(1),
	PreRun: func(cmd *cobra.Command, args []string) {
		util.RequireLogin()
	},
	Run: func(cmd *cobra.Command, args []string) {
		projectId := args[0]

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

		// Validate the project exists
		project, err := api.CallGetProjectById(httpClient, projectId)
		if err != nil {
			util.HandleError(err, "Unable to find project. Make sure the project ID is correct")
		}

		// Write the workspace config file (same pattern as init.go writeWorkspaceFile)
		workspaceFileToSave := models.WorkspaceConfigFile{
			WorkspaceId: projectId,
		}

		marshalledWorkspaceFile, err := json.MarshalIndent(workspaceFileToSave, "", "    ")
		if err != nil {
			util.HandleError(err, "Unable to save project configuration")
		}

		err = util.WriteToFile(util.INFISICAL_WORKSPACE_CONFIG_FILE_NAME, marshalledWorkspaceFile, 0600)
		if err != nil {
			util.HandleError(err, "Unable to write workspace config file")
		}

		util.PrintSuccessMessage(fmt.Sprintf("Switched to project: %s (%s)", project.Name, project.ID))

		Telemetry.CaptureEvent("cli-command:projects switch",
			posthog.NewProperties().Set("version", util.CLI_VERSION))
	},
}

var projectsDescribeCmd = &cobra.Command{
	Use:     "describe",
	Short:   "Show details of the current or specified project",
	Example: "infisical projects describe\ninfisical projects describe --projectId=ID",
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

		// Get project info
		project, err := api.CallGetProjectById(httpClient, projectId)
		if err != nil {
			util.HandleError(err, "Unable to fetch project details")
		}

		// Get environments
		envResponse, err := api.CallGetAccessibleEnvironments(httpClient, api.GetAccessibleEnvironmentsRequest{
			WorkspaceId: projectId,
		})

		outputFormat, _ := cmd.Flags().GetString("output")
		if outputFormat != "" {
			envList := []map[string]any{}
			if err == nil {
				for _, env := range envResponse.AccessibleEnvironments {
					envList = append(envList, map[string]any{
						"name":          env.Name,
						"slug":          env.Slug,
						"isWriteDenied": env.IsWriteDenied,
					})
				}
			}
			outputStructure := map[string]any{
				"id":           project.ID,
				"name":         project.Name,
				"slug":         project.Slug,
				"environments": envList,
			}
			output, err := util.FormatOutput(outputFormat, outputStructure, nil)
			if err != nil {
				util.HandleError(err, "Unable to format output")
			}
			util.PrintStdout(output)
		} else {
			// Print project info
			util.PrintfStdout("Project: %s\n", project.Name)
			util.PrintfStdout("ID:      %s\n", project.ID)
			util.PrintfStdout("Slug:    %s\n\n", project.Slug)

			if err != nil {
				util.PrintfStdout("Environments: unable to fetch (%s)\n", err.Error())
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
				util.PrintfStdout("Environments:\n")
				visualize.GenericTable(headers, rows)
			}
		}

		Telemetry.CaptureEvent("cli-command:projects describe",
			posthog.NewProperties().Set("version", util.CLI_VERSION))
	},
}

func init() {
	projectsListCmd.Flags().String("org-id", "", "Filter projects by organization ID")
	util.AddOutputFlagsToCmd(projectsListCmd, "The output format for projects")

	projectsDescribeCmd.Flags().String("projectId", "", "The project ID to describe (defaults to current)")
	util.AddOutputFlagsToCmd(projectsDescribeCmd, "The output format for project details")

	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsSwitchCmd)
	projectsCmd.AddCommand(projectsDescribeCmd)
	RootCmd.AddCommand(projectsCmd)
}
