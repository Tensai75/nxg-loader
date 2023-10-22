package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

func par2() error {

	Log.Info("Starting repair process")

	var (
		par2ExitCodes  map[int]string
		par2FileName   string
		parameters     []string
		cmdReader      io.ReadCloser
		scanner        *bufio.Scanner
		parProgressBar *progressbar.ProgressBar
		err            error
	)

	// par2 exit codes
	par2ExitCodes = map[int]string{
		0: "Success",
		1: "Repair possible",
		2: "Repair not possible",
		3: "Invalid command line arguments",
		4: "Insufficient critical data to verify",
		5: "Repair failed",
		6: "FileIO Error",
		7: "Logic Error",
		8: "Out of memory",
	}

	exp, _ := regexp.Compile(`^.+\.par2`)
	if err = filepath.Walk(conf.TempPath, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && exp.MatchString(filepath.Base(info.Name())) {
			par2FileName = info.Name()
		}
		return nil
	}); err != nil {
		checkForFatalErr(err)
	}

	// set parameters
	parameters = append(parameters, "r", "-q")
	if conf.DeletePar2 {
		parameters = append(parameters, "-p")
	}
	parameters = append(parameters, filepath.Join(conf.TempPath, par2FileName))

	cmd := exec.Command(conf.Par2Exe, parameters...)
	Log.Debug("Par command: %s", cmd.String())
	if conf.Debug || conf.Verbose > 0 {
		// create a pipe for the output of the program
		if cmdReader, err = cmd.StdoutPipe(); err != nil {
			return err
		}
		scanner = bufio.NewScanner(cmdReader)
		scanner.Split(scanLines)
		go func() {
			// progress bar
			if conf.Verbose > 0 {
				parProgressBar = progressbar.NewOptions(int(100),
					progressbar.OptionSetDescription("INFO:    Repairing files    "),
					progressbar.OptionSetRenderBlankState(true),
					progressbar.OptionThrottle(time.Millisecond*100),
					progressbar.OptionShowElapsedTimeOnFinish(),
					progressbar.OptionOnCompletion(newline),
				)
			}

			for scanner.Scan() {
				output := strings.Trim(scanner.Text(), " \r\n")
				if output != "" && !strings.Contains(output, "%") {
					Log.Debug("PAR: %v", output)
				}
				if conf.Verbose > 0 {
					exp := regexp.MustCompile(`(\d+)\.?\d*%`)
					if output != "" && exp.MatchString(output) {
						percentStr := exp.FindStringSubmatch(output)
						percentInt, _ := strconv.Atoi(percentStr[1])
						parProgressBar.Set(percentInt)
					}
				}
			}

		}()
	}
	if err = cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if errMsg, ok := par2ExitCodes[exitError.ExitCode()]; ok {
				return fmt.Errorf(errMsg)
			} else {
				return fmt.Errorf("Unknown error")
			}
		}
	}
	if conf.Verbose > 0 {
		parProgressBar.Finish()
	}
	Log.Info("Repair successful")

	return nil
}

func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		if data[i] == '\n' {
			// We have a line terminated by single newline.
			return i + 1, data[0:i], nil
		}
		advance = i + 1
		if len(data) > i+1 && data[i+1] == '\n' {
			advance += 1
		}
		return advance, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}
