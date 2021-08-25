//go:generate packr2

package terraform

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"get.porter.sh/porter/pkg/context" // We are not using go-yaml because of serialization problems with jsonschema, don't use this library elsewhere
	"github.com/gobuffalo/packr/v2"
	"github.com/pkg/errors"
)

const (
	// DefaultWorkingDir is the default working directory for Terraform.
	DefaultWorkingDir = "terraform"

	// DefaultClientVersion is the default version of the terraform cli.
	DefaultClientVersion = "1.0.4"

	// DefaultInitFile is the default file used to initialize terraform providers during build.
	DefaultInitFile = ""
)

// terraform is the logic behind the terraform mixin
type Mixin struct {
	*context.Context
	schema *packr.Box
	config MixinConfig
}

// New terraform mixin client, initialized with useful defaults.
func New() *Mixin {
	return &Mixin{
		Context: context.New(),
		schema:  packr.New("schema", "./schema"),
		config: MixinConfig{
			WorkingDir:    DefaultWorkingDir,
			ClientVersion: DefaultClientVersion,
			InitFile:      DefaultInitFile,
		},
	}
}

func (m *Mixin) getPayloadData() ([]byte, error) {
	reader := bufio.NewReader(m.In)
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "could not read the payload from STDIN")
	}
	return data, nil
}

func (m *Mixin) getOutput(outputName string) ([]byte, error) {
	cmd := m.NewCommand("terraform", "output", "-raw", outputName)
	cmd.Stderr = m.Err

	// Terraform appears to auto-append a newline character when printing outputs
	// Trim this character before returning the output
	out, err := cmd.Output()
	out = bytes.TrimRight(out, "\n")

	if err != nil {
		prettyCmd := fmt.Sprintf("%s %s", cmd.Path, strings.Join(cmd.Args, " "))
		return nil, errors.Wrap(err, fmt.Sprintf("couldn't run command %s", prettyCmd))
	}

	return out, nil
}

func (m *Mixin) handleOutputs(outputs []Output) error {
	for _, output := range outputs {
		bytes, err := m.getOutput(output.Name)
		if err != nil {
			return err
		}

		err = m.Context.WriteMixinOutputToFile(output.Name, bytes)
		if err != nil {
			return errors.Wrapf(err, "unable to write output '%s'", output.Name)
		}
	}
	return nil
}

// commandPreRun runs setup tasks applicable for every action
func (m *Mixin) commandPreRun(step *Step) error {
	if step.LogLevel != "" {
		os.Setenv("TF_LOG", step.LogLevel)
	}

	// First, change to specified working dir
	m.Chdir(m.config.WorkingDir)
	if m.Debug {
		fmt.Fprintln(m.Err, "Terraform working directory is", m.Getwd())
	}

	// Initialize Terraform
	fmt.Println("Initializing Terraform...")
	err := m.Init(step.BackendConfig)
	if err != nil {
		return fmt.Errorf("could not init terraform, %s", err)
	}
	return nil
}
