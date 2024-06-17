package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type BuildError struct {
	Error string
	Line  string
	File  string
	Col   string
}

func (e BuildError) Open() {
	exec.Command("code", "-g", e.Location()).Run()
}

func (e BuildError) Location() string {
	return e.File + ":" + e.Line + ":" + e.Col
}

func main() {
	closeOnNoError := flag.Bool("e", false, "close when no errors")
	// useShortCuts := flag.Bool("h", false, "hotkey for next error")
	shouldLog := flag.Bool("v", false, "Verbose log events")
	shouldLogOnErrorFix := flag.Bool("logonfix", false, "Log on error fixed")
	buildCmd := flag.String("cmd", "build", "Cmd to use for next error choices are (build|test|run-test)")
	containsFiles := flag.String("contains", "// XXX", "use this to change what value is looked to be in a line, if it is in the line it is counted as an error")

	flag.Parse()
	if !*shouldLog {
		log.SetOutput(io.Discard)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	defer func() { _ = w.Close() }()

	move := make(chan struct{}, 2)

	errs := GetListOfErrors(*buildCmd, *containsFiles)
	currentLocation, pos := GetFirstError(errs, w, *closeOnNoError)

	for currentLocation == nil {
		fmt.Printf("\t✅\r")
		time.Sleep(time.Millisecond * 100)
		errs = GetListOfErrors(*buildCmd, *containsFiles)
		currentLocation, pos = GetFirstError(errs, w, *closeOnNoError)
	}

	log.Println(pos)
	go func() {
		for range w.Events {
			log.Println("Got event")
			move <- struct{}{}
		}
	}()

	// if *useShortCuts {
	// 	go shortCuts(errs, pos)
	// 	defer robotgo.End()
	// }

	ticker := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-move:
			log.Println("Moving to next error")
		case <-ticker.C:
			log.Println("Tick")
		}

		errs = GetListOfErrors(*buildCmd, *containsFiles)
		if len(errs) == 0 {
			if *closeOnNoError {
				return
			}
			fmt.Printf("\t✅\r")
			continue
		}
		log.Println(errs)
		fmt.Printf("\t🔥\t%d\r", len(errs))
		if len(errs) > 0 && !currentErrorInErrors(currentLocation, errs) {
			if *shouldLogOnErrorFix {
				fmt.Println("Fixed Error:", currentLocation.Location())
			}
			w.Remove(currentLocation.File)
			currentLocation = errs[0]
			pos = 0
			currentLocation.Open()
			err := w.Add(currentLocation.File)
			if err != nil {
				panic(err)
			}

		}
	}
}

func currentErrorInErrors(currentLocation *BuildError, errs []*BuildError) bool {
	for _, e := range errs {
		if e.Location() == currentLocation.Location() {
			return true
		}
	}
	return false
}
func GetFirstError(errs []*BuildError, w *fsnotify.Watcher, closeOnNoError bool) (*BuildError, int) {
	if len(errs) > 0 {
		currentLocation := errs[0]
		pos := 0
		currentLocation.Open()
		err := w.Add(currentLocation.File)
		if err != nil {
			fmt.Println(currentLocation.File)
			panic(err)
		}
		return currentLocation, pos
	} else {
		if closeOnNoError {
			os.Exit(0)
		}
	}
	return nil, 0
}

func GetXXXErrors(contains string) []*BuildError {
	errs, err := findLinesContaining(".", contains, []string{".go"})
	if err != nil {
		log.Printf("Failed to find lines:  %v", err)
	}
	return errs
}

func findLinesContaining(directory string, prefix string, extensions []string) ([]*BuildError, error) {
	var result []*BuildError

	// Walk through the directory
	err := filepath.WalkDir(directory, func(path string, info os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// If it's a file (not a directory), process it
		if !info.IsDir() {
			found := false
			for _, ext := range extensions {
				if strings.HasSuffix(path, ext) {
					found = true
				}
				if !found {
					return nil
				}
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNumber := 0
			for scanner.Scan() {
				lineNumber++
				line := scanner.Text()
				if strings.Contains(line, prefix) {
					be := &BuildError{
						File:  path,
						Line:  fmt.Sprintf("%d", lineNumber),
						Error: line,
						Col:   fmt.Sprintf("%d", strings.Index(line, prefix)),
					}
					result = append(result, be)
				}
			}

			if err := scanner.Err(); err != nil {
				if err == io.EOF {
					return nil
				}
				log.Printf("failed to scan file: %q %v", path, err)
				return nil
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetListOfErrors(buildCmd string, contains string) []*BuildError {
	var out []byte
	var err error
	errs := GetXXXErrors(contains)

	switch buildCmd {
	case "run-test":
		out, err = exec.Command(`go`, "test", "-count=1", "./...").CombinedOutput()
	case "test":
		out, err = exec.Command(`go`, "test", "-exec", "/bin/true", "./...").CombinedOutput()
	case "build":
		out, err = exec.Command(`go`, "build", "-o", "/tmp/nexterrorBinTest").CombinedOutput()
	case "notes":
		return errs
	default:
		return nil
	}
	log.Println(string(out))
	if err == nil {
		return errs
	}
	r := bufio.NewReader(bytes.NewReader(out))
	for {
		l, _, err := r.ReadLine()
		if err == io.EOF {
			if len(errs) > 0 {
				log.Println(errs[0].Location())
			}
			return errs
		}
		if err != nil {
			panic(err)
		}
		vals := strings.SplitN(string(l), ":", 4)
		switch len(vals) {
		case 3:
			if strings.Contains(vals[0], "Error Trace") {
				errs = append([]*BuildError{
					{
						File: strings.TrimSpace(vals[1]),
						Line: strings.TrimSpace(vals[2]),
					},
				}, errs...)
			}
		case 4:
			errs = append([]*BuildError{
				{
					File:  vals[0],
					Line:  vals[1],
					Col:   vals[2],
					Error: vals[3],
				},
			}, errs...)
		}
	}
}

// func shortCuts(errs []*BuildError, pos int) {
// 	evts := robotgo.Start()
// 	log.Println("Starting event loop")
// 	for e := range evts {
// 		switch {
// 		case e.Keychar == 65535 && e.Mask == 40964:
// 			pos++
// 			if pos > len(errs) {
// 				pos = 0
// 			}
// 			if pos < len(errs)-1 {
// 				errs[pos].Open()
// 			}
// 		default:
// 			log.Println(string(e.Keychar), e.Mask)
// 		}
// 	}
// }
