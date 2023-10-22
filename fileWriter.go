package main

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/chrisfarms/yenc"
)

type FileChannels struct {
	channels map[string]chan *yenc.Part
}

type FileWriters struct {
	sync.Mutex
	writers map[string]bool
}

func (fileWriter *FileWriters) runOnce(name string) {
	fileWriter.Lock()
	defer fileWriter.Unlock()
	if _, ok := fileWriter.writers[name]; ok {
		return
	} else {
		fileChannels.channels[name] = make(chan *yenc.Part, conf.Connections*2)
		fileWriterWG.Add(1)
		go writeFile(fileChannels.channels[name], name, &fileWriterWG)
		fileWriter.writers[name] = true
	}
}

var (
	fileChannels FileChannels
	fileWriters  FileWriters
)

func writeFile(parts <-chan *yenc.Part, name string, wg *sync.WaitGroup) {

	Log.Debug("Start writing file \"%v\"", name)

	defer wg.Done()

	var (
		destFile *os.File
		err      error
	)

	if destFile, err = os.OpenFile(filepath.Join(conf.TempPath, name), os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		Log.Error("WRITER: Unable to create file \"%v\": %v", conf.TempPath, err)
		exit(1)
	}
	defer destFile.Close()

	for {

		part, ok := <-parts
		if !ok {
			return
		}
		if _, err = destFile.WriteAt(part.Body, part.Begin-1); err != nil {
			Log.Warn("Unable to write bytes %v to %v to destination file \"%v\": %v", part.Begin-1, part.Begin-1+int64(len(part.Body)), part.Name, err)
		}
		if conf.Verbose > 0 {
			downloadProgressBar.Add(1)
		}
	}

}
