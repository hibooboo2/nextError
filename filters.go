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

func useFile(file string) bool {
	fileNameContainsList := []string{
		"vendor",
		"_test.go",
		"/snap/go/",
		"../",
		"/pkg/mod/",
	}
	for _, v := range fileNameContainsList {
		if strings.Contains(file, v) {
			return false
		}
	}
	return true
}
