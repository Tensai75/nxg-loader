package main

import (
	"sync"

	"github.com/chrisfarms/yenc"
)

func yEncEncoder(wg *sync.WaitGroup) {

	defer wg.Done()

	for {

		article, ok := <-articlesChan
		if !ok {
			break
		}

		var (
			part *yenc.Part
			err  error
		)

		if part, err = yenc.Decode(article.Body); err != nil {
			Log.Warn("Unable to decode body of message id %v: %v", article.Header["Message-ID"][0], err)
			missingParts.add(article.Header["Message-ID"][0])
			continue
		} else {
			fileWriters.runOnce(part.Name)
			fileChannels.channels[part.Name] <- part
		}
	}

}
