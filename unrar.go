package main

import (
	"bufio"
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

func unrar() error {

	Log.Info("Starting unrar process")

	var (
		rarExitCodes   map[int]string
		parameters     []string
		cmdReader      io.ReadCloser
		scanner        *bufio.Scanner
		rarProgressBar *progressbar.ProgressBar
		err            error
	)

	// rar exit codes
	rarExitCodes = map[int]string{
		0:   "Successful operation",
		1:   "Warning. Non fatal error(s) occurred",
		2:   "A fatal error occurred",
		3:   "Invalid checksum. Data is damaged",
		4:   "Attempt to modify a locked archive",
		5:   "Write error",
		6:   "File open error",
		7:   "Wrong command line option",
		8:   "Not enough memory",
		9:   "File create error",
		10:  "No files matching the specified mask and options were found",
		11:  "Wrong password",
		255: "User break",
	}

	// set parameters
	parameters = append(parameters, "x", "-o+")
	if conf.Password != "" {
		parameters = append(parameters, fmt.Sprintf("-p%v", conf.Password))
	}
	parameters = append(parameters, filepath.Join(conf.TempPath, "*.rar"))
	parameters = append(parameters, conf.DestPath)

	cmd := exec.Command(conf.RarExe, parameters...)
	Log.Debug("Unrar command: %s", cmd.String())
	if conf.Debug || conf.Verbose > 0 {
		// create a pipe for the output of the program
		if cmdReader, err = cmd.StdoutPipe(); err != nil {
			return err
		}
		scanner = bufio.NewScanner(cmdReader)
		go func() {
			// progress bar
			if conf.Verbose > 0 {
				rarProgressBar = progressbar.NewOptions(int(100),
					progressbar.OptionSetDescription("INFO:    Extracting files   "),
					progressbar.OptionSetRenderBlankState(true),
					progressbar.OptionThrottle(time.Millisecond*100),
					progressbar.OptionShowElapsedTimeOnFinish(),
					progressbar.OptionOnCompletion(newline),
				)
			}

			for scanner.Scan() {
				output := strings.Trim(scanner.Text(), " \r\n")
				if conf.Debug {
					if output != "" && !strings.Contains(output, "%") {
						Log.Debug("RAR: %v", output)
					}
				}
				if conf.Verbose > 0 {
					exp := regexp.MustCompile(`0*(\d+)%`)
					if output != "" && exp.MatchString(output) {
						percentStr := exp.FindStringSubmatch(output)
						percentInt, _ := strconv.Atoi(percentStr[1])
						rarProgressBar.Set(percentInt)
					}
				}
			}
		}()
	}
	if err = cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() > 1 {
			if errMsg, ok := rarExitCodes[exitError.ExitCode()]; ok {
				return fmt.Errorf(errMsg)
			} else {
				return fmt.Errorf("Unknown error")
			}
		}
	}
	if conf.Verbose > 0 {
		rarProgressBar.Finish()
	}
	Log.Info("Unrar successful")
	if conf.DeleteRar {
		Log.Info("Deleting the rar files")
		if err = filepath.Walk(conf.TempPath, func(file string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && filepath.Ext(file) == ".rar" {
				if err = os.Remove(file); err != nil {
					Log.Warn("Unable to remove rar file \"%v\": %v", file, err)
				}
			}
			return nil
		}); err != nil {
			checkForFatalErr(err)
		}
	}

	return nil
}
