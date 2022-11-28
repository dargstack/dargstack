package core

import (
	"log"
	"os"
	"strings"
)

func GetProjectNameAndOwner() (directory, project, owner string) {
	cwd, e := os.Getwd()
	if e != nil {
		log.Fatal(e)
	}

	cwdArr := strings.Split(cwd, "/")
	l := len(cwdArr)
	if l < 2 || !strings.HasSuffix(cwdArr[l-1], "_stack") {
		log.Fatal("Incorrect directory architecture! Must follow \"../<owner>/<project_name>_stack\" naming convention.")
	}

	directory = strings.ToValidUTF8(cwdArr[l-1], "-")
	project = strings.TrimSuffix(directory, "_stack")
	owner = strings.ToValidUTF8(cwdArr[l-2], "-")

	return
}
