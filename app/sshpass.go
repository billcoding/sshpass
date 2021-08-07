package app

import (
	"fmt"
	"github.com/creack/pty"
	"github.com/riywo/loginshell"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Options struct {
	ExpectedPrompt              string        // ExpectedPrompt is the string to treat as the password prompt e.g. "Password: "
	ExpectedFailure             string        // ExpectedFailure is the string to treat as an indication of failure e.g. "Incorrect password"
	Timeout                     time.Duration // Timeout is the duration to wait for a prompt/failure before giving up
	AutoConfirmHostAuthenticity bool          // AutoConfirmHostAuthenticity will type 'yes' on host authenticity confirmations when set
	Shell                       string        // Shell is a path to the shell to use e.g. /bin/bash - leave blank to use user shell
}

var DefaultOptions = &Options{
	ExpectedPrompt:              "password:",
	ExpectedFailure:             "denied",
	Timeout:                     time.Second * 10,
	AutoConfirmHostAuthenticity: true,
	Shell:                       "",
}

// Run attempts to run the provided command and insert the given passwords one by one when prompted.
func Run(cmd string, passwords []string, options *Options) error {
	if options == nil {
		options = DefaultOptions
	}

	shell := options.Shell

	if shell == "" {
		var err error
		shell, err = loginshell.Shell()
		if err != nil {
			shell = "/bin/bash"
		}
	}

	c := exec.Command(shell)

	pt, err := pty.Start(c)
	if err != nil {
		return err
	}
	defer func() { _ = pt.Close() }()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, pt); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGHUP

	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer func() { _ = terminal.Restore(int(os.Stdin.Fd()), oldState) }()

	if _, err := pt.Write([]byte(cmd + "; exit\n")); err != nil {
		return err
	}

	var buf string
	for i, password := range passwords {
		append, err := enterPassword(
			pt,
			password,
			options,
			i == len(passwords)-1,
			buf,
		)
		if err != nil {
			return err
		}
		buf = buf + append
	}

	return nil
}

func enterPassword(pt *os.File, password string, options *Options, redirectPipes bool, buffered string) (string, error) {

	errChan := make(chan error)
	readyChan := make(chan string)

	go func(data string) {
		buf := make([]byte, 4096)
		confirmed := false
		entered := false
		for {
			n, err := pt.Read(buf)
			if err != nil {
				errChan <- err
				break
			}
			if n == 0 {
				continue
			}
			data += string(buf[:n])
			if !confirmed && strings.Contains(data, "The authenticity of host ") {
				if options.AutoConfirmHostAuthenticity {
					confirmed = true
					data = ""
					pt.Write([]byte("yes\n"))
				} else {
					errChan <- fmt.Errorf("host authenticity confirmation required, but it was disabled")
					break
				}
			} else if !entered && strings.Contains(data, options.ExpectedPrompt) {
				entered = true
				data = ""
				pt.Write([]byte(password + "\n"))
			} else if entered && len(data) > 5 {
				if strings.Contains(data, options.ExpectedPrompt) || strings.Contains(data, options.ExpectedFailure) {
					errChan <- fmt.Errorf("authentication failure")
					break
				}
				readyChan <- data
				break
			}
		}
	}(buffered)

	timer := time.NewTimer(options.Timeout)
	defer timer.Stop()

	select {
	case newBuffered := <-readyChan:
		if redirectPipes {
			os.Stdout.WriteString(newBuffered)
			go func() { _, _ = io.Copy(pt, os.Stdin) }()
			_, _ = io.Copy(os.Stdout, pt)
			return "", nil
		}
		return newBuffered, nil
	case err := <-errChan:
		return "", err
	case <-timer.C:
		return "", fmt.Errorf("timed out waiting for prompt")
	}
}
