package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/dargstack/dargstack/v4/internal/resource"
)

// printIssues prints validation issues grouped by severity and then by
// description, so repeated problems across many resources are collapsed into a
// single entry with an indented resource list rather than N identical lines.
//
// Returns true when at least one error is present.
func printIssues(issues []resource.Issue) bool {
	var errs, warns []resource.Issue
	for _, iss := range issues {
		if iss.Severity == "error" {
			errs = append(errs, iss)
		} else {
			warns = append(warns, iss)
		}
	}

	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "%s\n", styleErr.Render(fmt.Sprintf("%d error(s):", len(errs))))
		printIssueGroup(errs)
	}
	if len(warns) > 0 {
		fmt.Fprintf(os.Stderr, "%s\n", styleWarn.Render(fmt.Sprintf("%d warning(s):", len(warns))))
		printIssueGroup(warns)
	}

	return len(errs) > 0
}

// printIssueGroup collapses issues that share the same description into a
// single entry and prints each group indented under the severity header.
func printIssueGroup(issues []resource.Issue) {
	type entry struct {
		resources []string
	}

	seen := make(map[string]*entry)
	var order []string

	for _, iss := range issues {
		if _, ok := seen[iss.Description]; !ok {
			seen[iss.Description] = &entry{}
			order = append(order, iss.Description)
		}
		seen[iss.Description].resources = append(seen[iss.Description].resources, iss.Resource)
	}

	for _, desc := range order {
		e := seen[desc]
		sort.Strings(e.resources)
		if len(e.resources) == 1 {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", e.resources[0], desc)
		} else {
			fmt.Fprintf(os.Stderr, "  %s\n", desc)
			for _, r := range e.resources {
				fmt.Fprintf(os.Stderr, "    · %s\n", r)
			}
		}
	}
	fmt.Fprintln(os.Stderr)
}
