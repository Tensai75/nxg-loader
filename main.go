package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Tensai75/nntp"
	"github.com/chrisfarms/yenc"
	"github.com/schollz/progressbar/v3"
)

type Messages struct {
	id      string
	retries int
}

type Counter struct {
	mu      sync.Mutex
	counter int64
}

func (c *Counter) inc() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counter++
	return c.counter
}

type MissingParts struct {
	mu    sync.Mutex
	parts []string
}

func (m *MissingParts) add(messageId string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parts = append(m.parts, messageId)
}

// global variables
var (
	appName        = "NxG Loader"
	appVersion     string
	logFileName    = "nxg-loader.log"
	configFileName = "nxg-loader.conf"

	appExec             string
	appPath             string
	homePath            string
	partsCounter        Counter
	missingParts        MissingParts
	decodedHeader       []byte
	maxDataParts        int
	maxPar2Parts        int
	downloadProgressBar *progressbar.ProgressBar
	err                 error

	// channels
	messagesChan chan Messages
	articlesChan chan nntp.Article

	// wait groups
	fileWriterWG   sync.WaitGroup
	readArticlesWG sync.WaitGroup
	yEncDecodeWG   sync.WaitGroup

	// counters
	failedConnections Counter
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
		maxDataParts, _ = strconv.Atoi(matches[0][1])
		Log.Debug("Total data parts: %v", maxDataParts)
		maxPar2Parts, _ = strconv.Atoi(matches[0][2])
		Log.Debug("Total par2 parts: %v", maxPar2Parts)
		loadArticles(maxDataParts, "data")
		Log.Info("Download of data files completed")
	} else {
		Log.Error("Provided header is invalid")
		exit(1)
	}

	if len(missingParts.parts) > 0 {
		Log.Info("Missing parts: %v", len(missingParts.parts))
		Log.Info("Downloaded files are incomplete and need to be repaired")
		if maxPar2Parts > 0 {
			loadArticles(maxPar2Parts, "par2")
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

func GetSHA256Hash(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

func checkForFatalErr(err error) {
	if err != nil {
		Log.Error(err.Error())
		exit(1)
	}
}

func loadArticles(maxParts int, partType string) {

	Log.Info("Loading %v files", partType)

	// progress bar
	if conf.Verbose > 0 {
		downloadProgressBar = progressbar.NewOptions(int(maxParts),
			progressbar.OptionSetDescription(fmt.Sprintf("INFO:    Loading %s files ", partType)),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionThrottle(time.Millisecond*100),
			progressbar.OptionShowElapsedTimeOnFinish(),
			progressbar.OptionOnCompletion(newline),
		)
	}

	// make channels
	messagesChan = make(chan Messages, conf.Connections*2)
	articlesChan = make(chan nntp.Article, conf.Connections*20)

	// empty files channel
	fileChannels.channels = nil
	fileWriters.writers = nil

	// initialise channels
	fileChannels.channels = make(map[string]chan *yenc.Part)
	fileWriters.writers = make(map[string]bool)

	// launche the go-routines
	for i := 1; i <= conf.Connections; i++ {
		readArticlesWG.Add(1)
		go readArticles(&readArticlesWG, i, 0)
		yEncDecodeWG.Add(1)
		go yEncEncoder(&yEncDecodeWG)
	}

	for j := 1; j <= maxParts; j++ {
		md5Hash := GetSHA256Hash(fmt.Sprintf("%v:%v:%v", conf.Header, partType, j))
		messageId := md5Hash[:40] + "@" + md5Hash[40:61] + "." + md5Hash[61:]
		messagesChan <- Messages{messageId, 0}
	}

	close(messagesChan)
	readArticlesWG.Wait()
	close(articlesChan)
	yEncDecodeWG.Wait()
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

func newline() { fmt.Println() }
