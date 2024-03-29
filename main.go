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
	buildCmd := flag.String("cmd", "build", "Cmd to use for next error choices are (build|test)")

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

	errs := GetListOfErrors(*buildCmd)
	currentLocation, pos := GetFirstError(errs, w, *closeOnNoError)

	for currentLocation == nil {
		fmt.Printf("\t✅\r")
		time.Sleep(time.Millisecond * 100)
		errs = GetListOfErrors(*buildCmd)
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

		errs = GetListOfErrors(*buildCmd)
		if len(errs) == 0 {
			if *closeOnNoError {
				return
			}
			fmt.Printf("\t✅\r")
			continue
		}
		fmt.Printf("\t🔥\r")
		if len(errs) > 0 && errs[0].Location() != currentLocation.Location() {
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

func GetListOfErrors(buildCmd string) []*BuildError {
	var out []byte
	var err error

	switch buildCmd {
	case "test":
		out, err = exec.Command(`go`, "test", "-exec", "/bin/true", "./...").CombinedOutput()
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
