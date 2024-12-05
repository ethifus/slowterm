/*
   Slowterm - Slow down your console
*/

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

var defaultCmd = []string{"/bin/bash"}
var defaultBaudrate = 9600 // 300, 600, 1200, 2400, 4800, 9600, 19200, 38400

func slowterm(baudrate int, cmd []string) error {
	c := exec.Command(cmd[0], cmd[1:]...)

	// Start the command with a pty
	ptmx, err := pty.Start(c)
	if err != nil {
		return err
	}
	// Make sure to close the pty at the end
	defer func() { _ = ptmx.Close() }()

	// Handle pty size change
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				fmt.Fprintf(os.Stderr, "error resizing pty: %s\n", err)
			}
		}
	}()
	defer func() { signal.Stop(ch); close(ch) }()

	// Initial resize
	ch <- syscall.SIGWINCH

	// Set stdin in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	var sleepDuration = time.Duration(1.0 / float64(baudrate) * float64(time.Second))

	// Copy stdin to the pty and the pty to stdout
	go copyWithDelay(ptmx, os.Stdin, sleepDuration)
	copyWithDelay(os.Stdout, ptmx, sleepDuration)

	return nil
}

func copyWithDelay(t, s *os.File, delay time.Duration) {
	for {
		var before = time.Now()
		if _, err := io.CopyN(t, s, 1); err != nil {
			break
		}

		var after = time.Now()
		var elapsed = after.Sub(before)
		var toSleep = delay - elapsed
		if toSleep.Seconds() > 0 {
			time.Sleep(toSleep)
		}
	}
}

func printHelp() {
	fmt.Println("NAME")
	fmt.Printf("\tslowterm - Slow down your console\n\n")
	fmt.Println("SYNOPSIS")
	fmt.Printf("\tslowterm [-b BAUDRATE] [COMMAND ARGS...]\n\n")
	fmt.Println("DESCRIPTION")
	fmt.Printf("\tRun shell or given command with emulated baudrate to slow down input\n")
	fmt.Printf("\tand output. Why? Because apparently suffering builds character.\n\n")
	fmt.Println("OPTIONS")
	fmt.Println("  -b BAUDRATE")
	fmt.Printf("\tSet emulated baudrate (default %d)\n\n", defaultBaudrate)
	fmt.Println("  COMMAND ARGS")
	fmt.Println("\tCommand to run with arguments. By default slowterm will try to run")
	fmt.Println("\tcurrent user default shell (from $SHELL) or /usr/bin/bash if $SHELL is")
	fmt.Println("\tis empty.\n\n")
}

func main() {
	var baudrate int
	var cmd []string
	var help bool

	flag.IntVar(&baudrate, "b", defaultBaudrate, "Baudrate")
	flag.BoolVar(&help, "help", false, "Show help")
	flag.Parse()

	if help {
		printHelp()
		return
	}

	if vals := flag.Args(); len(vals) > 0 {
		cmd = vals
	} else {
		if shell := os.Getenv("SHELL"); shell != "" {
			cmd = []string{shell}
		} else {
			cmd = defaultCmd
		}
	}

	if err := slowterm(baudrate, cmd); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
