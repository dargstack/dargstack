package core

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/joho/godotenv"
)

func DockerSudo(args ...string) []byte {
	mode := args[0]
	args = args[1:]

	if isDockerUser() {
		c := exec.Command("docker", args...)
		c.Stderr = os.Stderr
		out, e := c.Output()
		if e != nil {
			log.Fatal(e)
		}
		return out
	}

	envVars := new(strings.Builder)
	envVars.WriteString("STACK_DOMAIN")

	if mode == "production" {
		envVarMap, e := godotenv.Read("./src/production/stack.env")
		if e != nil {
			fmt.Printf(
				"Not preserving any environment variables for root as file\n%s",
				Hi1("./src/production/stack.env was not found.\n"),
			)
		} else {
			for k := range envVarMap {
				envVars.WriteString(",")
				envVars.WriteString(k)
			}
		}
	} else {
		envVarMap, err := godotenv.Read("./src/development/stack.env")
		if err != nil {
			fmt.Printf(
				"Not preserving any environment variables for root as file\n%s",
				Hi1("./src/development/stack.env was not found.\n"),
			)
		} else {
			for k := range envVarMap {
				envVars.WriteString(",")
				envVars.WriteString(k)
			}
		}
	}

	args = append([]string{"--preserve-env=" + envVars.String(), "docker"}, args...)

	c := exec.Command("sudo", args...)
	c.Stderr = os.Stderr
	out, err := c.Output()
	if err != nil {
		log.Fatal(err)
	}
	return out
}

// Check if user is in docker group
func isDockerUser() bool {
	currentUser, e1 := user.Current()
	if e1 != nil {
		log.Fatal(e1)
	}
	dockerGroup, e2 := user.LookupGroup("docker")
	if e2 != nil {
		log.Fatal(e2)
	}
	groupsOfUser, e3 := currentUser.GroupIds()
	if e3 != nil {
		log.Fatal(e3)
	}
	for _, gid := range groupsOfUser {
		if gid == dockerGroup.Gid {
			return true
		}
	}
	return false
}

func IsStackRunning(project string) bool {
	serviceRunning := DockerSudo("default", "service", "ls", "--filter", "label=com.docker.stack.namespace="+project, "-q")
	networkRunning := DockerSudo("default", "network", "ls", "--filter", "label=com.docker.stack.namespace="+project, "-q")
	return string(serviceRunning) != "" || string(networkRunning) != ""
}
