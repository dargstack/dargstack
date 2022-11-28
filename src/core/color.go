package core

import "github.com/fatih/color"

func Hi2(str string) string {
	return color.New(color.FgYellow, color.Bold).Sprint(str)
}

func Hi1(str string) string {
	return color.New(color.FgHiBlue, color.Bold).Sprint(str)
}

func Err(str string) string {
	return color.New(color.FgRed, color.Bold).Sprint(str)
}

func Succ(str string) string {
	return color.New(color.FgHiGreen, color.Bold).Sprint(str)
}
