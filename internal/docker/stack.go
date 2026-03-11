package docker

import (
	"fmt"
	"strings"
)

// StackDeploy deploys a stack from compose YAML data.
func StackDeploy(exec *Executor, stackName string, composeData []byte) error {
	// docker stack deploy -c - <stack_name>
	// Feed compose data via stdin
	err := exec.RunWithStdin(composeData, "stack", "deploy", "-c", "-", stackName)
	if err != nil {
		return fmt.Errorf("deploy stack %q: %w", stackName, err)
	}
	return nil
}

// StackRemove removes a deployed stack.
func StackRemove(exec *Executor, stackName string) error {
	_, err := exec.Run("stack", "rm", stackName)
	if err != nil {
		return fmt.Errorf("remove stack %q: %w", stackName, err)
	}
	return nil
}

// ServiceList lists service names in a stack, returning only the bare service name
// (without the "<stack>_" prefix).
func ServiceList(exec *Executor, stackName string) ([]string, error) {
	out, err := exec.Run("service", "ls", "--filter", fmt.Sprintf("label=com.docker.stack.namespace=%s", stackName), "--format", "{{.Name}}")
	if err != nil {
		return nil, fmt.Errorf("list services in stack %q: %w", stackName, err)
	}
	if out == "" {
		return nil, nil
	}
	prefix := stackName + "_"
	raw := strings.Split(out, "\n")
	names := make([]string, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		names = append(names, strings.TrimPrefix(s, prefix))
	}
	return names, nil
}

// ServiceRemove removes specific services by their full Docker service names (<stack>_<service>).
func ServiceRemove(exec *Executor, serviceNames []string) error {
	if len(serviceNames) == 0 {
		return nil
	}
	args := append([]string{"service", "rm"}, serviceNames...)
	_, err := exec.Run(args...)
	if err != nil {
		return fmt.Errorf("remove services: %w", err)
	}
	return nil
}

// VolumeList lists volumes with an optional filter.
func VolumeList(exec *Executor, stackName string) ([]string, error) {
	out, err := exec.Run("volume", "ls", "--filter", fmt.Sprintf("label=com.docker.stack.namespace=%s", stackName), "--format", "{{.Name}}")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// VolumeRemove removes the specified volumes.
func VolumeRemove(exec *Executor, volumes []string) error {
	if len(volumes) == 0 {
		return nil
	}
	args := append([]string{"volume", "rm"}, volumes...)
	_, err := exec.Run(args...)
	return err
}

// StackBuild builds a service image from a Dockerfile.
func StackBuild(exec *Executor, contextPath, target, tag string) error {
	args := []string{"build", "--target", target, "-t", tag, contextPath}
	return exec.RunPassthrough(args...)
}
