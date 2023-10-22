package main

func defaultConfig() string {
	return `# Usenet server settings
# Usenet server host name or IP address
Host: "news.newshosting.com"
# Usenet server port number
Port: 119
# Use SSL if set to true
SSL: false
# Username to connect to the usenet server
NntpUser: ""
# Password to connect to the usenet server
NntpPass: ""
# Number of connections to use to connect to the usenet server
Connections: 50
# Number of retries upon connection error
ConnRetries: 3
# Time to wait in seconds before trying to re-connect
ConnWaitTime: 5
# Number of retries before article reading fails
Retries: 3

# Par2 settings
# Repair files
Repair: true
# Delete par2 files after successful repair or if no repair needed
DeletePar2: true
# Absolute path to par2cmdline exe (https://github.com/animetosho/par2cmdline-turbo/releases)
Par2Exe: "C:\\Tools\\Par2\\par2.exe"

# Rar settings
# Unrar
Unrar: true
# Delete rar files after successful unrar
DeleteRar: true
# Absolute path to unrar exe (https://www.rarlab.com/rar_add.htm)
RarExe: "C:\\Tools\\Rar\\unrar.exe"

# Path settings
# All paths must be absolut paths or are treated as relative paths to the user's home folder
# Temporary path for downloaded files (if left empty default temp path is used)
TempPath: "D:/loader/Temp"
# Final destination path for the downloaded files
DestPath: "D:/loader/Downloads"
# Path for the log file (leave empty to disable logging)
LogFilePath: "D:/loader/Logs"

# Verbosity level of cmd output
# 0 = no output except for fatal errors
# 1 = outputs information
# 2 = outputs information and non-fatal errors (warnings)
# 3 = outputs information, non-fatal errors (warnings) and additional debug information if activated
Verbose: 2

# Debug mode (logs additional debug information)
Debug: true

# Miscellaneous settings
# Wait for the programm to end (close the window)
EndWaitTime: true
# Wait time for the programm to end (close the window) after success
SuccessWaitTime: 3
# Wait time for the programm to end (close the window) after an error occured
ErrorWaitTime: 15	
`
}
