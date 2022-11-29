package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/azolus/dargstack/core"
)

const (
	devPathRel  = "src/development"
	prodPathRel = "src/production"
)

type stack struct {
	developmentPath string
	productionPath  string
	seds            []string
}

// `deriveCmd` represents the derive command
var deriveCmd = &cobra.Command{
	Use:   "derive",
	Short: "Derives a ./production/stack.yml from ./development/stack.yml.",
	Long:  "Derives a ./production/stack.yml from ./development/stack.yml.",
	Run: func(cmd *cobra.Command, args []string) {
		stack := stack{}
		stack.readPaths()
		stack.readSeds()

		devStackYaml := stack.developmentPath + "/stack.yml"
		prodStackYaml := stack.productionPath + "/stack.yml"

		// copy stack.yml from development to production
		fmt.Printf("Deriving production %s from development %s...\n",
			core.Hi1("stack.yml"), core.Hi1("stack.yml"))
		copyYamlFile(
			devStackYaml,
			prodStackYaml,
		)

		// applying seds
		err := applySeds(stack.seds, prodStackYaml)
		if err != nil {
			log.Fatalf("%s applying %s commands to %s: %v",
				core.Err("error"), core.Hi1("search and replace"), core.Hi1("production.yml"), err)
		}

		_, err = os.Stat(stack.productionPath + "/production.yml")
		if err != nil && !os.IsNotExist(err) {
			log.Fatalf("%s merging %s: %v",
				core.Err("error"), core.Hi1("production.yml"), err)
		} else if err == nil {
			fmt.Printf("Merging %s...\n",
				core.Hi1(fmt.Sprintf("%s%s", stack.productionPath, "/production.yml")))
			out := core.DockerSudo(
				"default", "run", "--rm", "-v",
				stack.productionPath+"//stack.yml:/manifests/stack.yml", "-v",
				stack.productionPath+"//production.yml:/manifests/production.yml",
				"gfranks/spruce", "spruce", "merge",
				"/manifests/stack.yml", "/manifests/production.yml")

			err := os.WriteFile(prodStackYaml, out, 0o755)
			if err != nil {
				log.Fatalf("%s writing to %s: %v", core.Err("error"), core.Hi1("stack.yml"), err)
			}
		} else {
			log.Fatalf("%s opening %s: %v", core.Err("error"), core.Hi1("production.yml"), err)
		}

		stack.deriveStackEnv()
		fmt.Println(core.Succ("Done."))
	},
}

func init() {
	rootCmd.AddCommand(deriveCmd)
}

// Used to copy the `stack.yml` file from development to production
func copyYamlFile(origin, destination string) {
	// Read yml file from development
	data, err := os.ReadFile(origin)
	if err != nil {
		log.Fatalf(
			"%s reading %s: %v",
			core.Err("error"), core.Hi1("origin"), err)
	}

	// Writing to destination
	os.WriteFile(destination, data, 0644)
	if err != nil {
		log.Fatalf(
			"%s writing to %s: %v",
			core.Err("error"), core.Hi1(destination), err)
	}
}

// Applies search and replace to file
func applySeds(seds []string, file string) error {
	for _, expr := range seds {
		c := exec.Command("sed", "-i", expr, file)
		_, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf(
				"%s executing search and replace for file %s: %v",
				core.Err("error"), core.Hi1(file), err)
		}
	}
	return nil
}

// Deriving the `production/stack.env` from development
func (stack *stack) deriveStackEnv() {
	devStackEnv := stack.developmentPath + "/stack.env"
	prodStackEnv := stack.productionPath + "/stack.env"
	prodProdEnv := stack.productionPath + "/production.env"
	prodStackEnvFile, err := os.OpenFile(
		prodStackEnv, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o777) // create or overwrite
	if err != nil {
		log.Fatalf("%s creating %s for production: %v", core.Err("error"), core.Hi1("stack.env"), err)
	}
	defer prodStackEnvFile.Close()

	// Delete contents of production stack.env (if it already exists)
	prodStackEnvFile.Truncate(0)

	// Append development/stack.env to production stack.env
	if _, err := os.Stat(devStackEnv); err == nil {
		fmt.Println("Adding development environment variables...")
		data, err := os.ReadFile(devStackEnv)
		if err != nil {
			log.Fatalf("%s reading %s in development: %v", core.Err("error"), core.Hi1("/stack.env"), err)
		}
		prodStackEnvFile.Write(data)
	}

	// Append production/production.env to production/stack.env
	if _, err := os.Stat(prodProdEnv); err == nil {
		fmt.Println("Adding production environment variables...")
		data, err := os.ReadFile(prodProdEnv)
		if err != nil {
			log.Fatalf("%s reading %s in production: %v", core.Err("error"), core.Hi1("/production.env"), err)
		}
		prodStackEnvFile.Write(data)
	}
}

// Setting path variables
func (stack *stack) readPaths() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("error reading current directory: %v", err)
	}

	stack.developmentPath = cwd + "/" + devPathRel
	stack.productionPath = cwd + "/" + prodPathRel
}

// Set default sed-commands
func (stack *stack) readSeds() {
	stack.seds = []string{
		`s/^.* #DARGSTACK-REMOVE$//g`,
		`s/file:.*\.secret/external: true/g`,
		`s/\.\/certificates\//acme_data/g`,
		`s/\.\.\/production/\./g`,
	}
}
