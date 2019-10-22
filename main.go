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

	"github.com/go-vgo/robotgo"

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
	var currentLocation BuildError
	pos := 0
	if len(errs) > 0 {
		currentLocation = errs[0]
		pos = 0
		currentLocation.Open()
		err = w.Add(currentLocation.File)
		if err != nil {
			panic(err)
		}
	}
	go func() {
		for _ = range w.Events {
			log.Println("Got event")
			move <- struct{}{}
		}
	}()
	evts := robotgo.Start()
	defer robotgo.End()
	go func() {
		log.Println("Starting event loop")
		for e := range evts {
			switch {
			case e.Keychar == 65535 && e.Mask == 40964:
				pos++
				if pos > len(errs) {
					pos = 0
				}
				if pos < len(errs)-1 {
					errs[pos].Open()
				}
			default:
				log.Println(string(e.Keychar), e.Mask)
			}
		}
	}()
	for _ = range move {
		log.Println("Updating build errors")
		errs = GetListOfErrors()
		time.Sleep(10 * time.Millisecond)
		if len(errs) == 0 {
			if *closeOnNoError {
				return
			}
			time.Sleep(time.Second * 5)
		}
		if len(errs) > 0 && errs[0].Location() != currentLocation.Location() {
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
