package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	parser "github.com/alexflint/go-arg"
)

// arguments structure
type Args struct {
	NxgLnk          string `arg:"positional" help:"Fully qualified NXGLNK URI (nxglnk://?h=header&t=title&p=password)"`
	Header          string `arg:"--header" help:"Header to be downloaded" placeholder:"STRING"`
	Password        string `arg:"--password" help:"Password to extract the downloaded rar file" placeholder:"STRING"`
	Title           string `arg:"--title" help:"Title of the download" placeholder:"STRING"`
	Register        bool   `arg:"--register" help:"Register the NXGLNK scheme"`
	Host            string `arg:"--host" help:"Usenet server host name or IP address" placeholder:"HOST"`
	Port            int    `arg:"--port" help:"Usenet server port number" placeholder:"INT"`
	SSL             bool   `arg:"-"`
	SSL_arg         string `arg:"--ssl" help:"Use SSL" placeholder:"true|false"`
	NntpUser        string `arg:"--user" help:"Username to connect to the usenet server" placeholder:"STRING"`
	NntpPass        string `arg:"--pass" help:"Password to connect to the usenet server" placeholder:"STRING"`
	Connections     int    `arg:"--connections" help:"Ammount of connections to use to connect to the usenet server" placeholder:"INT"`
	ConnRetries     int    `arg:"--connretries" help:"Number of retries upon connection error" placeholder:"INT"`
	ConnWaitTime    int    `arg:"--connwaittime" help:"Time to wait in seconds before trying to re-connect" placeholder:"INT"`
	Retries         int    `arg:"--retries" help:"Number of retries before article reading fails" placeholder:"INT"`
	Repair          bool   `arg:"-"`
	Repair_arg      string `arg:"--repair" help:"Repair downloaded files using the par2 files" placeholder:"true|false"`
	DeletePar2      bool   `arg:"-"`
	DeletePar2_arg  string `arg:"--delpar2" help:"Delete par2 files after successful repair or if no repair needed" placeholder:"true|false"`
	Par2Exe         string `arg:"--par2exe" help:"Path to the par2.exe" placeholder:"PATH"`
	Unrar           bool   `arg:"-"`
	Unrar_arg       string `arg:"--unrar" help:"Automatically extract the downloaded rar files" placeholder:"true|false"`
	DeleteRar       bool   `arg:"-"`
	DeleteRar_arg   string `arg:"--delrar" help:"Delete rar files after successful unrar" placeholder:"true|false"`
	RarExe          string `arg:"--rarexe" help:"Path to the unrar.exe" placeholder:"PATH"`
	TempPath        string `arg:"--temp" help:"Temporary path for the downloaded files" placeholder:"PATH"`
	DestPath        string `arg:"--dest" help:"Final destination path for the downloaded files" placeholder:"PATH"`
	LogFilePath     string `arg:"--log" help:"Path for the log file" placeholder:"PATH"`
	Verbose         int    `arg:"--verbose" help:"Verbosity level of cmd output" placeholder:"0-3"`
	Debug           bool   `arg:"-"`
	Debug_arg       string `arg:"--debug" help:"Activate debug mode" placeholder:"true|false"`
	Test            string `arg:"--test" help:"Activate test mode and read messages from PATH instead from usenet" placeholder:"PATH"`
	EndWaitTime     bool   `arg:"-"`
	SuccessWaitTime int    `arg:"-"`
	ErrorWaitTime   int    `arg:"-"`
}

// version information
func (Args) Version() string {
	return "\n" + appName + " " + appVersion + "\n"
}

// additional description
func (Args) Epilogue() string {
	return "\nParameters that are passed as arguments have precedence over the settings in the configuration file and the NXGLNK (if provided).\n"
}

// parser variable
var argParser *parser.Parser

func parseArguments() {

	parserConfig := parser.Config{
		IgnoreEnv: true,
	}

	// parse flags
	argParser, _ = parser.NewParser(parserConfig, &conf)
	if err := parser.Parse(&conf); err != nil {
		if err.Error() == "help requested by user" {
			writeHelp(argParser)
			fmt.Println(conf.Epilogue())
			os.Exit(0)
		} else if err.Error() == "version requested by user" {
			fmt.Println(conf.Version())
			os.Exit(0)
		}
		writeUsage(argParser)
		Log.Error(err.Error())
		os.Exit(1)
	}

}

func checkArguments() {

	// check --register flag
	if conf.Register {
		registerProtocol()
	}

	if conf.Header == "" && conf.NxgLnk == "" {
		Log.Error("You must provide either the --header argument or a NXGLNK URI")
		os.Exit(1)
	}

	// parse nxglnk if provided
	if conf.NxgLnk != "" {
		if nxglnk, err := url.Parse(conf.NxgLnk); err == nil {
			if nxglnk.Scheme == "nxglnk" {
				if query, err := url.ParseQuery(nxglnk.RawQuery); err == nil {
					if h := query.Get("h"); h != "" && conf.Header == "" {
						conf.Header = strings.TrimSpace(h)
					} else {
						writeUsage(argParser)
						Log.Error("Invalid NXGLNK URI: missing 'h' parameter")
						os.Exit(1)
					}
					if t := query.Get("t"); t != "" && conf.Title == "" {
						conf.Title = strings.TrimSpace(t)
					}
					if p := query.Get("p"); p != "" && conf.Password == "" {
						conf.Password = strings.TrimSpace(p)
					}
				}
			} else {
				writeUsage(argParser)
				Log.Error("Invalid NXGLNK URI")
				os.Exit(1)
			}
		}
	}

	// check paths
	if conf.DestPath == "" {
		writeUsage(argParser)
		Log.Error("No destination path provided")
		os.Exit(1)
	}
	if conf.TempPath == "" {
		conf.TempPath = os.TempDir()
	}

	if !filepath.IsAbs(conf.TempPath) {
		if conf.TempPath, err = filepath.Abs(filepath.Join(homePath, conf.TempPath)); err != nil {
			Log.Error("Unable to determine temporary path: ", err)
			os.Exit(1)
		}
	}
	if !filepath.IsAbs(conf.DestPath) {
		if conf.DestPath, err = filepath.Abs(filepath.Join(homePath, conf.DestPath)); err != nil {
			Log.Error("Unable to determine destination path: ", err)
			os.Exit(1)
		}
	}
	if conf.TempPath == conf.DestPath {
		Log.Error("Temporary path and destination path must be different")
		os.Exit(1)
	}
	if conf.Title != "" {
		// sanitize title
		exp := regexp.MustCompile(`[\\/:*?"<>|]`)
		conf.TempPath = filepath.Join(conf.TempPath, exp.ReplaceAllString(conf.Title, ""))
		conf.DestPath = filepath.Join(conf.DestPath, exp.ReplaceAllString(conf.Title, ""))
	} else {
		conf.TempPath = filepath.Join(conf.TempPath, conf.Header)
		conf.DestPath = filepath.Join(conf.DestPath, conf.Header)
	}

	// check bools
	if conf.SSL_arg != "" {
		if conf.SSL_arg == "true" {
			conf.SSL = true
		} else if conf.SSL_arg == "false" {
			conf.SSL = false
		}
	}
	if conf.Repair_arg != "" {
		if conf.Repair_arg == "true" {
			conf.Repair = true
		} else if conf.Repair_arg == "false" {
			conf.Repair = false
		}
	}
	if conf.DeletePar2_arg != "" {
		if conf.DeletePar2_arg == "true" {
			conf.DeletePar2 = true
		} else if conf.DeletePar2_arg == "false" {
			conf.DeletePar2 = false
		}
	}
	if conf.Unrar_arg != "" {
		if conf.Unrar_arg == "true" {
			conf.Unrar = true
		} else if conf.Unrar_arg == "false" {
			conf.Unrar = false
		}
	}
	if conf.DeleteRar_arg != "" {
		if conf.DeleteRar_arg == "true" {
			conf.DeleteRar = true
		} else if conf.DeleteRar_arg == "false" {
			conf.DeleteRar = false
		}
	}
	if conf.Debug_arg != "" {
		if conf.Debug_arg == "true" {
			conf.Debug = true
		} else if conf.Debug_arg == "false" {
			conf.Debug = false
		}
	}
}

func writeUsage(parser *parser.Parser) {
	var buf bytes.Buffer
	parser.WriteUsage(&buf)
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
}

func writeHelp(parser *parser.Parser) {
	var buf bytes.Buffer
	parser.WriteHelp(&buf)
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
}
