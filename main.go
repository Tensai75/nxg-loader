package main

import (
	"encoding/base64"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chrisfarms/yenc"
	"github.com/schollz/progressbar/v3"
)

// global variables
var (
	appName        = "NxG Loader"
	appVersion     string
	logFileName    = "nxg-loader.log"
	configFileName = "nxg-loader.conf"

	appExec             string
	appPath             string
	homePath            string
	decodedHeader       []byte
	totalParts          = make(map[string]int, 2)
	downloadProgressBar *progressbar.ProgressBar
	err                 error

	// wait groups
	fileWriterWG   sync.WaitGroup
	readArticlesWG sync.WaitGroup

	// counters
	failedConnections atomic.Int64
	totalBytesLoaded  atomic.Int64
	totalPartsLoaded  atomic.Int64
)

func init() {
	// set path variables
	if appExec, err = os.Executable(); err != nil {
		Log.Error("Unable to determin application path")
		exit(1)
	}
	appPath = filepath.Dir(appExec)
	if homePath, err = os.UserHomeDir(); err != nil {
		Log.Error("Unable to determin home path")
		exit(1)
	}

	// change working directory
	// important for url protocol handling (otherwise work dir will be system32 on windows)
	if err := os.Chdir(appPath); err != nil {
		Log.Error("Cannot change working directory: ", err)
		os.Exit(1)
	}
}

func main() {
	setConfPath()
	loadConfig()
	// init logger
	if conf.LogFilePath != "" {
		if !filepath.IsAbs(conf.LogFilePath) {
			conf.LogFilePath = filepath.Join(homePath, conf.LogFilePath)
		}
		initLogger(conf.LogFilePath)
		defer logClose()
	}
	parseArguments()
	checkArguments()

	// decode header
	if decodedHeader, err = base64.StdEncoding.DecodeString(conf.Header); err != nil {
		Log.Error("Provided header is invalid")
		exit(1)
	}

	// make paths
	if err = os.MkdirAll(conf.TempPath, os.ModePerm); err != nil {
		Log.Error("Unable to create temporary path \"%v\": %v", conf.TempPath, err)
		exit(1)
	}
	if err = os.MkdirAll(conf.DestPath, os.ModePerm); err != nil {
		Log.Error("Unable to create destination path \"%v\": %v", conf.DestPath, err)
		exit(1)
	}

	exp, err := regexp.Compile(`.+:(\d+):(\d+)`)
	if err != nil {
		Log.Error("%v", err)
	}
	if exp.MatchString(string(decodedHeader)) {
		matches := exp.FindAllStringSubmatch(string(decodedHeader), -1)
		totalParts["data"], _ = strconv.Atoi(matches[0][1])
		Log.Debug("Total data parts: %v", totalParts["data"])
		totalParts["par2"], _ = strconv.Atoi(matches[0][2])
		Log.Debug("Total par2 parts: %v", totalParts["par2"])
		loadArticles("data")
		Log.Info("Download of data files completed")
	} else {
		Log.Error("Provided header is invalid")
		exit(1)
	}

	if len(missingArticles.parts) > 0 {
		Log.Info("Missing parts: %v", len(missingArticles.parts))
		Log.Info("Downloaded files are incomplete and need to be repaired")
		if totalParts["par2"] > 0 {
			loadArticles("par2")
			Log.Info("Download of par2 files completed")
			if conf.Repair {
				if err = par2(); err != nil {
					Log.Error("Error while repairing: %v", err)
					moveFiles()
					exit(1)
				}
			}
		} else {
			Log.Error("No par2 files provided. Repair not possible.")
			moveFiles()
			exit(1)
		}
	}

	if conf.Unrar {
		if err = unrar(); err != nil {
			Log.Error("Error while extracting rar archive: %v", err)
		}
	}

	moveFiles()
	exit(0)

}

func loadArticles(partType string) {

	Log.Info("Loading %v files", partType)

	// progress bar
	if conf.Verbose > 0 {
		// initially set the target to max int64, we will estimate the "correct" target later
		downloadProgressBar = progressbar.NewOptions64(math.MaxInt64,
			progressbar.OptionSetDescription("INFO:    Downloading        "),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionShowElapsedTimeOnFinish(),
			progressbar.OptionOnCompletion(newline),
		)
	}

	// empty files channel
	fileChannels.channels = nil
	fileWriters.writers = nil

	// initialise channels
	fileChannels.channels = make(map[string]chan *yenc.Part)
	fileWriters.writers = make(map[string]bool)

	// empty counters
	totalBytesLoaded.Store(0)
	totalPartsLoaded.Store(0)

	// launche the go-routines
	for i := 1; i <= conf.Connections; i++ {
		readArticlesWG.Add(1)
		go readArticles(&readArticlesWG, i, 0)
	}

	for j := 1; j <= totalParts[partType]; j++ {
		md5Hash := GetSHA256Hash(fmt.Sprintf("%v:%v:%v", conf.Header, partType, j))
		messageId := md5Hash[:40] + "@" + md5Hash[40:61] + "." + md5Hash[61:]
		articlesChan <- Article{messageId, 0, partType}
	}

	close(articlesChan)
	readArticlesWG.Wait()
	for _, channel := range fileChannels.channels {
		close(channel)
	}
	fileWriterWG.Wait()
	if conf.Verbose > 0 {
		downloadProgressBar.Finish()
	}
	return
}

func moveFiles() {
	Log.Info("Moving files to \"%v\"", conf.DestPath)
	if err = filepath.WalkDir(conf.TempPath, func(filePath string, dir fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !dir.IsDir() {
			if err = os.Rename(filePath, filepath.Join(conf.DestPath, filepath.Base(filePath))); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		Log.Error("Error while moving files from \"%v\" to \"%v\": %v", conf.TempPath, conf.DestPath, err)
		exit(1)
	}
}

// always use exit function to terminate
// cmd window will stay open for the configured time if the program was startet outside a cmd window
func exit(exitCode int) {

	// clean up
	Log.Debug("Deleting temporary folder \"%v\"", conf.TempPath)
	if err = os.RemoveAll(conf.TempPath); err != nil {
		Log.Warn("Error while deleting temporary folder: %v", err)
	}

	if exitCode > 0 {
		Log.Error("Download failed")
	} else {
		Log.Succ("Download successful")
	}

	if conf.EndWaitTime {
		waitTime := conf.SuccessWaitTime
		if exitCode > 0 {
			waitTime = conf.ErrorWaitTime
		}

		// pause before ending the program
		fmt.Println()
		for i := waitTime; i >= 0; i-- {
			fmt.Print("\033[G\033[K") // move the cursor left and clear the line
			fmt.Printf("Ending program in %d seconds %s", i, strings.Repeat(".", waitTime-i))
			if i > 0 {
				time.Sleep(1 * time.Second)
			}
		}
		fmt.Println()
	}

	os.Exit(exitCode)
}
