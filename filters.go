package main

import "strings"

func useError(err string) bool {
	errorMessageContainsList := []string{
		"does not escape",
		"leaking param",
	}
	for _, v := range errorMessageContainsList {
		if strings.Contains(err, v) {
			return false
		}
	}
	return true
}

func useFile(file string, includeTests bool) bool {
	fileNameContainsList := []string{
		"vendor",
		"/snap/go/",
		"../",
		"/pkg/mod/",
	}
	if !includeTests {
		fileNameContainsList = append(fileNameContainsList, "_test.go")
	}
	for _, v := range fileNameContainsList {
		if strings.Contains(file, v) {
			return false
		}
	}
	return true
}
