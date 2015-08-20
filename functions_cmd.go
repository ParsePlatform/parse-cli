package main

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type functionHook struct {
	FunctionName string `json:"functionName,omitempty"`
	URL          string `json:"url,omitempty"`
	Warning      string `json:"warning,omitempty"`
}

func (f functionHook) String() string {
	if f.URL != "" {
		return fmt.Sprintf("Function name: %q, URL: %q", f.FunctionName, f.URL)
	}
	return fmt.Sprintf("Function name: %q", f.FunctionName)
}

type functionHooksCmd struct {
	All      bool
	Function *functionHook
}

func readFunctionName(e *env) (*functionHook, error) {
	var f functionHook
	fmt.Fprintf(e.Out, "Please enter the function name: ")
	fmt.Fscanf(e.In, "%s\n", &f.FunctionName)
	if f.FunctionName == "" {
		return nil, errors.New("Function name cannot be empty")
	}
	return &f, nil
}

func readFunctionParams(e *env) (*functionHook, error) {
	f, err := readFunctionName(e)
	if err != nil {
		return nil, err
	}

	fmt.Fprint(e.Out, "URL: https://")
	fmt.Fscanf(e.In, "%s\n", &f.URL)
	f.URL = "https://" + f.URL
	if err := validateURL(f.URL); err != nil {
		return nil, err
	}

	return f, nil
}

const defaultFunctionsURL = "/1/hooks/functions"

func (h *functionHooksCmd) functionHooksCreate(e *env, ctx *context) error {
	params, err := readFunctionParams(e)
	if err != nil {
		return err
	}
	var res functionHook
	functionsURL, err := url.Parse(defaultFunctionsURL)
	if err != nil {
		return err
	}
	_, err = e.ParseAPIClient.Post(functionsURL, params, &res)
	if err != nil {
		return err
	}
	if res.Warning != "" {
		fmt.Fprintf(e.Err, "WARNING: %s\n", res.Warning)
	}

	fmt.Fprintf(e.Out,
		"Successfully created a webhook function %q pointing to %q\n",
		res.FunctionName,
		res.URL,
	)
	return nil
}

func (h *functionHooksCmd) functionHooksRead(e *env, ctx *context) error {
	u := defaultFunctionsURL
	var function *functionHook
	if !h.All {
		funct, err := readFunctionName(e)
		if err != nil {
			return err
		}
		function = funct
		u = path.Join(u, function.FunctionName)
	}
	functionsURL, err := url.Parse(u)
	if err != nil {
		return err
	}

	var res struct {
		Results []*functionHook `json:"results,omitempty"`
	}
	_, err = e.ParseAPIClient.Get(functionsURL, &res)
	if err != nil {
		return err
	}
	var output []string
	for _, function := range res.Results {
		output = append(output, function.String())
	}
	sort.Strings(output)

	if h.All {
		fmt.Fprintln(e.Out, "The following cloudcode or webhook functions are associated with this app:")
	} else {
		if len(output) == 1 {
			fmt.Fprintf(e.Out, "You have one function named: %q\n", function.FunctionName)
		} else {
			fmt.Fprintf(e.Out, "The following functions named: %q are associated with your app:\n", function.FunctionName)
		}
	}
	fmt.Fprintln(e.Out, strings.Join(output, "\n"))
	return nil
}

func (h *functionHooksCmd) functionHooksUpdate(e *env, ctx *context) error {
	params, err := readFunctionParams(e)
	if err != nil {
		return err
	}
	var res functionHook
	functionsURL, err := url.Parse(path.Join(defaultFunctionsURL, params.FunctionName))
	if err != nil {
		return err
	}

	_, err = e.ParseAPIClient.Put(functionsURL, &functionHook{URL: params.URL}, &res)
	if err != nil {
		return err
	}
	if res.Warning != "" {
		fmt.Fprintf(e.Err, "WARNING: %s\n", res.Warning)
	}

	fmt.Fprintf(e.Out,
		"Successfully update the webhook function %q to point to %q\n",
		res.FunctionName,
		res.URL,
	)
	return nil
}

func (h *functionHooksCmd) functionHooksDelete(e *env, ctx *context) error {
	params, err := readFunctionName(e)
	if err != nil {
		return err
	}
	functionsURL, err := url.Parse(path.Join(defaultFunctionsURL, params.FunctionName))
	if err != nil {
		return err
	}

	confirmMessage := fmt.Sprintf(
		"Are you sure you want to delete webhook function: %q (y/n): ",
		params.FunctionName,
	)

	var res functionHook
	if getConfirmation(confirmMessage, e) {
		_, err = e.ParseAPIClient.Put(functionsURL, map[string]interface{}{"__op": "Delete"}, &res)
		if err != nil {
			return err
		}
		fmt.Fprintf(e.Out, "Successfully deleted webhook function %q\n", params.FunctionName)
		if res.FunctionName != "" {
			fmt.Fprintf(e.Out, "Function %q defined in cloud code will be used henceforth\n", res.FunctionName)
		}
	}

	return nil
}

func (h *functionHooksCmd) functionHooks(e *env, c *context) error {
	hp := *h
	hp.All = true
	return hp.functionHooksRead(e, c)
}

func newFunctionHooksCmd(e *env) *cobra.Command {
	var h functionHooksCmd

	c := &cobra.Command{
		Use:   "functions",
		Short: "List cloud code functions and function webhooks",
		Long:  "List cloud code functions and function webhooks",
		Run:   runWithClient(e, h.functionHooks),
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a function webhook",
		Long:  "Create a function webhook",
		Run:   runWithClient(e, h.functionHooksCreate),
	}
	c.AddCommand(createCmd)

	changeCmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit the URL of a function webhook",
		Long:  "Edit the URL of a function webhook",
		Run:   runWithClient(e, h.functionHooksUpdate),
	}
	c.AddCommand(changeCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a function webhook",
		Long:  "Delete a function webhook",
		Run:   runWithClient(e, h.functionHooksDelete),
	}
	c.AddCommand(deleteCmd)

	return c
}
