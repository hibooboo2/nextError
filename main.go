package main

import (
	"bufio"
	"bytes"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os/exec"
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
	shouldLog := flag.Bool("v", false, "Verbose log events")
	flag.Parse()
	if !*shouldLog {
		log.SetOutput(ioutil.Discard)
	}
	w, err := fsnotify.NewWatcher()
	defer w.Close()
	if err != nil {
		panic(err)
	}
	move := make(chan struct{}, 2)
	errs := GetListOfErrors()
	currentLocation := errs[0]
	currentLocation.Open()
	err = w.Add(currentLocation.File)
	if err != nil {
		panic(err)
	}
	go func() {
		for _ = range w.Events {
			log.Println("Got event")
			move <- struct{}{}
		}
	}()
	for _ = range move {
		log.Println("Updating build errors")
		errs = GetListOfErrors()
		time.Sleep(10 * time.Millisecond)
		if len(errs) == 0 {
			break
		}
		if errs[0].Location() != currentLocation.Location() {
			w.Remove(currentLocation.File)
			currentLocation = errs[0]
			currentLocation.Open()
			err := w.Add(currentLocation.File)
			if err != nil {
				panic(err)
			}

		}
	}
}

func GetListOfErrors() []BuildError {
	out, err := exec.Command(`go`, "build").CombinedOutput()
	if err == nil {
		return nil
	}
	r := bufio.NewReader(bytes.NewReader(out))
	errs := []BuildError{}
	for {
		l, _, err := r.ReadLine()
		if err == io.EOF {
			return errs
		}
		if err != nil {
			panic(err)
		}
		vals := strings.Split(string(l), ":")
		if len(vals) != 4 {
			continue
		}
		errs = append(errs, BuildError{
			File:  vals[0],
			Line:  vals[1],
			Col:   vals[2],
			Error: vals[3],
		})
	}
}
