package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-vgo/robotgo"
)

type BuildError struct {
	Error string
	Line  string
	File  string
	Col   string
}

func (e BuildError) Open(ide string) {
	switch ide {
	case "goland":
		exec.Command("goland", "--line", e.Line, e.File).Run()
	case "vscode":
		exec.Command("code", "-g", e.Location()).Run()
	default:
		panic(fmt.Sprintf("invalid ide %q", ide))
	}
}

func (e BuildError) Location() string {
	return e.File + ":" + e.Line + ":" + e.Col
}

func main() {
	closeOnNoError := flag.Bool("e", false, "close when no errors")
	useShortCuts := flag.Bool("h", false, "hotkey for next error")
	shouldLog := flag.Bool("v", false, "Verbose log events")
	shouldLogOnErrorFix := flag.Bool("logonfix", false, "Log on error fixed")
	buildCmd := flag.String("cmd", "build", "Cmd to use for next error choices are (build|test)")
	ide := flag.String("ide", "vscode", "choose which ide to use (vscode|goland)")

	flag.Parse()
	if !*shouldLog {
		log.SetOutput(ioutil.Discard)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	defer func() { _ = w.Close() }()

	move := make(chan struct{}, 2)

	errs := GetListOfErrors(*buildCmd)
	currentLocation, pos := GetFirstError(errs, w, *closeOnNoError, *ide)

	for currentLocation == nil {
		time.Sleep(time.Millisecond * 100)
		errs = GetListOfErrors(*buildCmd)
		currentLocation, pos = GetFirstError(errs, w, *closeOnNoError, *ide)
	}

	log.Println(pos)
	go func() {
		for range w.Events {
			log.Println("Got event")
			move <- struct{}{}
		}
	}()

	if *useShortCuts {
		go shortCuts(errs, pos, *ide)
		defer robotgo.End()
	}

	for range move {
		log.Println("Updating build errors")
		errs = GetListOfErrors(*buildCmd)
		if len(errs) == 0 {
			if *closeOnNoError {
				return
			}
			time.Sleep(time.Second * 5)
			continue
		}
		if len(errs) > 0 && errs[0].Location() != currentLocation.Location() {
			if *shouldLogOnErrorFix {
				fmt.Println("Fixed Error:", currentLocation.Location())
			}
			w.Remove(currentLocation.File)
			currentLocation = errs[0]
			pos = 0
			currentLocation.Open(*ide)
			err := w.Add(currentLocation.File)
			if err != nil {
				panic(err)
			}

		}
	}
}

func GetFirstError(errs []*BuildError, w *fsnotify.Watcher, closeOnNoError bool, ide string) (*BuildError, int) {
	if len(errs) > 0 {
		currentLocation := errs[0]
		pos := 0
		currentLocation.Open(ide)
		err := w.Add(currentLocation.File)
		if err != nil {
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

func GetListOfErrors(buildCmd string) []*BuildError {
	var out []byte
	var err error

	switch buildCmd {
	case "test":
		out, err = exec.Command(`go`, "test", "./...").CombinedOutput()
	case "build":
		out, err = exec.Command(`go`, "build", "-o", "/tmp/nexterrorBinTest").CombinedOutput()
	default:
		return nil
	}
	log.Println(string(out))
	if err == nil {
		return nil
	}
	r := bufio.NewReader(bytes.NewReader(out))
	errs := []*BuildError{}
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
		if len(vals) != 4 {
			continue
		}
		errs = append(errs, &BuildError{
			File:  vals[0],
			Line:  vals[1],
			Col:   vals[2],
			Error: vals[3],
		})
	}
}

func shortCuts(errs []*BuildError, pos int, ide string) {
	evts := robotgo.Start()
	log.Println("Starting event loop")
	for e := range evts {
		switch {
		case e.Keychar == 65535 && e.Mask == 40964:
			pos++
			if pos > len(errs) {
				pos = 0
			}
			if pos < len(errs)-1 {
				errs[pos].Open(ide)
			}
		default:
			log.Println(string(e.Keychar), e.Mask)
		}
	}
}
