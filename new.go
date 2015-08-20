package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/facebookgo/parse"
	"github.com/facebookgo/stackerr"
	"github.com/spf13/cobra"
)

type newCmd struct {
	addApplication bool
}

var newProjectFiles = []struct {
	dirname, filename, content string
}{
	{"cloud", "main.js", sampleSource},
	{"public", "index.html", sampleHTML},
}

func (n *newCmd) curlCommand(app *app) string {
	return fmt.Sprintf(
		`curl -X POST \
 -H "X-Parse-Application-Id: %s" \
 -H "X-Parse-REST-API-Key: %s" \
 -H "Content-Type: application/json" \
 -d '{}' \
 https://api.parse.com/1/functions/hello
`,
		app.ApplicationID,
		app.RestKey,
	)
}

func (n *newCmd) cloudCodeHelpMessage(e *env, app *app) string {
	return fmt.Sprintf(
		`Your Cloud Code has been created at %s.
Next, you might want to deploy this code with "parse deploy".
This includes a "Hello world" cloud function, so once you deploy
you can test that it works, with:

%s
`,
		e.Root,
		n.curlCommand(app),
	)
}

func (n *newCmd) getCloudCodeDir(e *env, appName string, isNew bool) (string, error) {
	var cloudCodeDir string
	fmt.Fprintf(e.Out,
		`Awesome! Now it's time to setup some Cloud Code for the app: %q,
Next we will create a directory to hold your Cloud Code.
Please enter the name to use for this directory,
or hit ENTER to use %q as the directory name.

Directory Name: `,
		appName,
		appName,
	)

	fmt.Scanf("%s\n", &cloudCodeDir)
	cloudCodeDir = strings.TrimSpace(cloudCodeDir)
	if cloudCodeDir == "" {
		cloudCodeDir = appName
	}

	helpMsg := "Next time, please select the same app you selected now."
	if isNew {
		helpMsg = `Next time, please select to add Cloud Code to an 'existing' app
and select the app you just created.`
	}

	configFile := filepath.Join(e.Root, cloudCodeDir, legacyConfigFile)
	if _, err := os.Lstat(configFile); err == nil {
		return "", stackerr.Newf(
			`Sorry, we are unable to create Cloud Code at %s.
It seems that you already have Cloud Code at %s.
Please run "parse new" again.
%s
Please choose a different name for your Cloud Code directory,
so it does not conflict with any other Cloud Code in the current directory.
`,
			cloudCodeDir,
			cloudCodeDir,
			helpMsg,
		)
	}

	cloudCodeDirInfo, err := os.Stat(filepath.Join(e.Root, cloudCodeDir))
	if err == nil {
		if !cloudCodeDirInfo.IsDir() {
			return "", stackerr.Newf(`Sorry, we are unable to create Cloud Code at %s.
In the current directory a file named: %s already exists.
Please run "parse new" again.
%s
Please choose a different name for your Cloud Code directory,
so it does not conflict with any other Cloud Code in the current directory.
`,
				cloudCodeDir,
				cloudCodeDir,
				helpMsg,
			)

		}
		return "", nil
	}
	if !os.IsNotExist(err) {
		return "", stackerr.Wrap(err)
	}
	return cloudCodeDir, nil
}

func (n *newCmd) createConfigWithContent(path, content string) error {
	file, err := os.OpenFile(
		path,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC,
		0600,
	)
	if err != nil && !os.IsExist(err) {
		return stackerr.Wrap(err)
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		return stackerr.Wrap(err)
	}
	if err := file.Close(); err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (n *newCmd) promptCreateNewApp(e *env) (string, error) {
	msg := `"new" and "existing" are the only valid options.
Please try again ...`

	var decision string
	for i := 0; i < 3; i++ {
		fmt.Fprintf(e.Out,
			`Would you like to create a new app, or add Cloud Code to an existing app?
Type "(n)ew" or "(e)xisting": `,
		)
		fmt.Fscanf(e.In, "%s\n", &decision)
		decision = strings.ToLower(decision)

		if decision == "new" || decision == "n" || decision == "existing" || decision == "e" {
			return decision, nil
		}
		fmt.Fprintln(e.Err, msg)
	}
	return "", stackerr.New(msg)
}

func (n *newCmd) setupSample(e *env, app *app, isNew bool) error {
	found := isProjectDir(getProjectRoot(e, e.Root))
	if !found {
		root := getLegacyProjectRoot(e, e.Root)
		_, err := os.Lstat(filepath.Join(root, legacyConfigFile))
		found = err == nil
	}
	if found {
		return stackerr.New(
			`Detected that you are already inside a Parse project.
Please refrain from creating a Parse project inside another Parse project.
`,
		)
	}

	switch e.Type {
	case parseFormat:
		return n.cloneSampleCloudCode(e, app, isNew)
	}
	return stackerr.Newf("Unknown project type: %d", e.Type)
}

func (n *newCmd) configureSample(
	add *addCmd,
	app *app,
	args []string,
	e *env,
) error {
	if err := add.addSelectedApp(app, args, e); err != nil {
		return err
	}

	// at this point user has already set a default app for the project
	// use its properties to fetch latest jssdk version and set it in config
	config, err := configFromDir(e.Root)
	if err != nil {
		return err
	}
	defaultApp, err := config.app(config.getDefaultApp())
	if err != nil {
		return err
	}
	masterKey, err := defaultApp.getMasterKey(e)
	if err != nil {
		return err
	}
	e.ParseAPIClient = e.ParseAPIClient.WithCredentials(
		parse.MasterKey{
			ApplicationID: defaultApp.getApplicationID(),
			MasterKey:     masterKey,
		},
	)

	if e.Type == parseFormat {
		return useLatestJSSDK(e)
	}
	return nil
}

func (n *newCmd) run(e *env) error {
	apps := &apps{}
	addCmd := &addCmd{MakeDefault: true, apps: apps}

	if err := apps.login.authUser(e); err != nil {
		return err
	}
	var (
		app *app
		err error
	)

	decision, err := n.promptCreateNewApp(e)
	if err != nil {
		return err
	}
	isNew := false

	switch decision {
	case "new", "n":
		isNew = true
		app, err = apps.createApp(e)
		if err != nil {
			return err
		}
	case "existing", "e":
		app, err = addCmd.selectApp(e)
		if err != nil {
			return err
		}
	}

	e.Type = parseFormat

	if err := n.setupSample(e, app, isNew); err != nil {
		return err
	}
	if err := n.configureSample(addCmd, app, nil, e); err != nil {
		return err
	}

	fmt.Fprintf(e.Out, n.cloudCodeHelpMessage(e, app))
	return nil
}

func newNewCmd(e *env) *cobra.Command {
	nc := newCmd{addApplication: true}
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Creates a new Parse app and adds Cloud Code to an existing Parse app",
		Long:  `Creates a new Parse app and adds Cloud Code to an existing Parse app.`,
		Run:   runNoArgs(e, nc.run),
	}
	return cmd
}
