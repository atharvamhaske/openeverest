// Copyright (C) 2026 The OpenEverest Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/rodaine/table"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/percona/everest/client"
)

const (
	apiDBDefaultServer = "http://localhost:8080/v1"
	apiDBDefaultNS     = "everest"
)

type apiDBConfig struct {
	Server string
	Token  string
	NS     string
	Output string
	File   string
}

var apiDBCfg = apiDBConfig{
	Server: apiDBDefaultServer,
	NS:     apiDBDefaultNS,
	Output: "table",
}

var apiDBCmd = &cobra.Command{
	Use:   "api-db <command> [flags]",
	Short: "POC database commands using the OpenEverest server API",
	Long:  "POC database commands using the OpenEverest server API. Use --ns for the OpenEverest namespace.",
}

var apiDBListCmd = &cobra.Command{
	Use:   "list [flags]",
	Args:  cobra.NoArgs,
	Short: "List database clusters through the OpenEverest API",
	RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := newAPIClient()
		if err != nil {
			return err
		}

		resp, err := c.ListDatabaseClustersWithResponse(cmd.Context(), apiDBCfg.NS)
		if err != nil {
			return err
		}
		if err := checkAPIResponse(resp.StatusCode(), resp.Body); err != nil {
			return err
		}
		list := resp.JSON200
		if list == nil {
			list = &client.DatabaseClusterList{}
			if err := unmarshalAPIBody(resp.Body, list); err != nil {
				return fmt.Errorf("parse database cluster list response: %w", err)
			}
		}
		return printDatabaseClusterList(list)
	},
}

var apiDBGetCmd = &cobra.Command{
	Use:   "get NAME [flags]",
	Args:  cobra.ExactArgs(1),
	Short: "Get a database cluster through the OpenEverest API",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newAPIClient()
		if err != nil {
			return err
		}

		resp, err := c.GetDatabaseClusterWithResponse(cmd.Context(), apiDBCfg.NS, args[0])
		if err != nil {
			return err
		}
		if err := checkAPIResponse(resp.StatusCode(), resp.Body); err != nil {
			return err
		}
		db := resp.JSON200
		if db == nil {
			db = &client.DatabaseCluster{}
			if err := unmarshalAPIBody(resp.Body, db); err != nil {
				return fmt.Errorf("parse database cluster response: %w", err)
			}
		}
		return printObject(db)
	},
}

var apiDBCreateCmd = &cobra.Command{
	Use:   "create -f FILE [flags]",
	Args:  cobra.NoArgs,
	Short: "Create a database cluster from YAML or JSON through the OpenEverest API",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if apiDBCfg.File == "" {
			return errors.New("required flag: -f, --file")
		}

		body, err := readFileOrStdin(apiDBCfg.File)
		if err != nil {
			return err
		}

		var db client.DatabaseCluster
		if err := yaml.Unmarshal(body, &db); err != nil {
			return fmt.Errorf("parse database cluster manifest: %w", err)
		}

		c, err := newAPIClient()
		if err != nil {
			return err
		}

		resp, err := c.CreateDatabaseClusterWithResponse(cmd.Context(), apiDBCfg.NS, db)
		if err != nil {
			return err
		}
		if err := checkAPIResponse(resp.StatusCode(), resp.Body); err != nil {
			return err
		}

		switch {
		case resp.JSON201 != nil:
			return printObject(resp.JSON201)
		case resp.JSON202 != nil:
			return printObject(resp.JSON202)
		case resp.JSON200 != nil:
			return printObject(resp.JSON200)
		default:
			if len(resp.Body) > 0 {
				db := &client.DatabaseCluster{}
				if err := unmarshalAPIBody(resp.Body, db); err != nil {
					return fmt.Errorf("parse database cluster create response: %w", err)
				}
				return printObject(db)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "database cluster create accepted: %s\n", resp.Status())
			return nil
		}
	},
}

var apiDBDeleteCmd = &cobra.Command{
	Use:   "delete NAME [flags]",
	Args:  cobra.ExactArgs(1),
	Short: "Delete a database cluster through the OpenEverest API",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newAPIClient()
		if err != nil {
			return err
		}

		resp, err := c.DeleteDatabaseClusterWithResponse(cmd.Context(), apiDBCfg.NS, args[0], &client.DeleteDatabaseClusterParams{})
		if err != nil {
			return err
		}
		if err := checkAPIResponse(resp.StatusCode(), resp.Body); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "database cluster %q deleted from namespace %q\n", args[0], apiDBCfg.NS)
		return nil
	},
}

var apiDBEnginesCmd = &cobra.Command{
	Use:   "engines [flags]",
	Args:  cobra.NoArgs,
	Short: "List database engines through the OpenEverest API",
	RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := newAPIClient()
		if err != nil {
			return err
		}

		resp, err := c.ListDatabaseEnginesWithResponse(cmd.Context(), apiDBCfg.NS)
		if err != nil {
			return err
		}
		if err := checkAPIResponse(resp.StatusCode(), resp.Body); err != nil {
			return err
		}
		list := resp.JSON200
		if list == nil {
			list = &client.DatabaseEngineList{}
			if err := unmarshalAPIBody(resp.Body, list); err != nil {
				return fmt.Errorf("parse database engine list response: %w", err)
			}
		}
		return printDatabaseEngineList(list)
	},
}

var apiDBLogsCmd = &cobra.Command{
	Use:   "logs NAME COMPONENT [flags]",
	Args:  cobra.ExactArgs(2),
	Short: "Fetch database component logs through the OpenEverest API",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := newAPIClient()
		if err != nil {
			return err
		}

		resp, err := c.GetDatabaseClusterComponentLogsWithResponse(
			cmd.Context(),
			apiDBCfg.NS,
			args[0],
			args[1],
			&client.GetDatabaseClusterComponentLogsParams{},
		)
		if err != nil {
			return err
		}
		if err := checkAPIResponse(resp.StatusCode(), resp.Body); err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(resp.Body)
		return err
	},
}

func init() {
	rootCmd.AddCommand(apiDBCmd)

	apiDBCmd.PersistentFlags().StringVar(&apiDBCfg.Server, "server", apiDBDefaultServer, "OpenEverest API server URL")
	apiDBCmd.PersistentFlags().StringVar(&apiDBCfg.Token, "token", "", "OpenEverest bearer token. If empty, EVEREST_TOKEN is used")
	apiDBCmd.PersistentFlags().StringVar(&apiDBCfg.NS, "ns", apiDBDefaultNS, "OpenEverest namespace")
	apiDBCmd.PersistentFlags().StringVarP(&apiDBCfg.Output, "output", "o", "table", "Output format: table, json, yaml")

	apiDBCreateCmd.Flags().StringVarP(&apiDBCfg.File, "file", "f", "", "DatabaseCluster YAML/JSON file, or - for stdin")

	apiDBCmd.AddCommand(apiDBListCmd)
	apiDBCmd.AddCommand(apiDBGetCmd)
	apiDBCmd.AddCommand(apiDBCreateCmd)
	apiDBCmd.AddCommand(apiDBDeleteCmd)
	apiDBCmd.AddCommand(apiDBEnginesCmd)
	apiDBCmd.AddCommand(apiDBLogsCmd)
}

func newAPIClient() (*client.ClientWithResponses, error) {
	token := apiDBCfg.Token
	if token == "" {
		token = os.Getenv("EVEREST_TOKEN")
	}

	opts := []client.ClientOption{}
	if token != "" {
		opts = append(opts, client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+token)
			return nil
		}))
	}
	return client.NewClientWithResponses(normalizeAPIServerURL(apiDBCfg.Server), opts...)
}

func normalizeAPIServerURL(server string) string {
	server = strings.TrimRight(server, "/")
	if strings.HasSuffix(server, "/v1") {
		return server
	}
	return server + "/v1"
}

func readFileOrStdin(path string) ([]byte, error) {
	if path == "-" {
		return os.ReadFile("/dev/stdin")
	}
	return os.ReadFile(path)
}

func checkAPIResponse(statusCode int, body []byte) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	var apiErr client.Error
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != nil {
		return fmt.Errorf("OpenEverest API returned %d: %s", statusCode, *apiErr.Message)
	}
	return fmt.Errorf("OpenEverest API returned %d: %s", statusCode, strings.TrimSpace(string(body)))
}

func unmarshalAPIBody(body []byte, dst any) error {
	if len(body) == 0 {
		return errors.New("empty response body")
	}
	if strings.HasPrefix(strings.TrimSpace(string(body)), "<") {
		return errors.New("received HTML from the OpenEverest UI; use the API base URL, for example --server http://localhost:8080/v1")
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("%w; body: %s", err, truncateBody(body))
	}
	return nil
}

func truncateBody(body []byte) string {
	const maxBodyLen = 500
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) <= maxBodyLen {
		return string(body)
	}
	return string(body[:maxBodyLen]) + "..."
}

func printDatabaseClusterList(list *client.DatabaseClusterList) error {
	if apiDBCfg.Output != "table" {
		return printObject(list)
	}

	tbl := table.New("NAME", "ENGINE", "VERSION", "STATUS")
	if list.Items != nil {
		for _, db := range *list.Items {
			engine, version, status := "", "", ""
			if db.Spec != nil {
				engine = string(db.Spec.Engine.Type)
				if db.Spec.Engine.Version != nil {
					version = *db.Spec.Engine.Version
				}
			}
			if db.Status != nil && db.Status.Status != nil {
				status = *db.Status.Status
			}
			tbl.AddRow(objectName(db.Metadata), engine, version, status)
		}
	}
	tbl.Print()
	return nil
}

func printDatabaseEngineList(list *client.DatabaseEngineList) error {
	if apiDBCfg.Output != "table" {
		return printObject(list)
	}

	tbl := table.New("NAME", "TYPE", "OPERATOR VERSION", "STATUS")
	if list.Items != nil {
		for _, engine := range *list.Items {
			engineType, operatorVersion, status := "", "", ""
			if engine.Spec != nil {
				engineType = engine.Spec.Type
			}
			if engine.Status != nil {
				if engine.Status.OperatorVersion != nil {
					operatorVersion = *engine.Status.OperatorVersion
				}
				if engine.Status.Status != nil {
					status = *engine.Status.Status
				}
			}
			tbl.AddRow(objectName(engine.Metadata), engineType, operatorVersion, status)
		}
	}
	tbl.Print()
	return nil
}

func printObject(v any) error {
	var (
		out []byte
		err error
	)

	switch apiDBCfg.Output {
	case "json":
		out, err = json.MarshalIndent(v, "", "  ")
	case "yaml":
		out, err = yaml.Marshal(v)
	case "table":
		out, err = json.MarshalIndent(v, "", "  ")
	default:
		return fmt.Errorf("unsupported output format %q", apiDBCfg.Output)
	}
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func objectName(metadata *map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	name, _ := (*metadata)["name"].(string)
	return name
}
