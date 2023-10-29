package main

import (
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chrisfarms/yenc"
)

type Article struct {
	id       string
	retries  int
	partType string
}

type MissingArticles struct {
	mu    sync.Mutex
	parts []string
}

func (m *MissingArticles) add(messageId string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parts = append(m.parts, messageId)
}

func (m *MissingArticles) len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.parts)
}

var (
	articleCounter  atomic.Int64
	missingArticles MissingArticles

	// channels
	articlesChan       = make(chan Article, 0)
	failedArticlesChan = make(chan Article, 0)
)

func init() {
	go failedArticlesHandler()
}

func failedArticlesHandler() {
	for {
		article, ok := <-failedArticlesChan
		if !ok {
			return
		}
		go func(article Article) {
			if err := TryCatch(func() { articlesChan <- article })(); err != nil {
				Log.Debug("Error while trying to add article with message id <%v> back to the queue: %v", article.id, err)
				missingArticles.add(article.id)
			} else {
				Log.Debug("Added article with message id <%v> back to the queue", article.id)
			}
		}(article)
	}
}

func readArticles(wg *sync.WaitGroup, connNumber int, retries int) {

	defer wg.Done()

	var conn *safeConn

	if retries > 0 {
		Log.Warn("Connection %d waiting %d seconds to reconnect", connNumber, conf.ConnWaitTime)
		time.Sleep(time.Second * time.Duration(conf.ConnWaitTime))
	}

	conn, err := ConnectNNTP()
	if err != nil {
		retries++
		if retries > conf.ConnRetries {
			Log.Error("Connection %d failed after %d retries: %v", connNumber, retries-1, err)
			failed := failedConnections.Add(1)
			if failed >= int64(conf.Connections) {
				checkForFatalErr(fmt.Errorf("All connections failed"))
			}
			return
		}
		Log.Warn("Connection %d error: %v", connNumber, err)
		wg.Add(1)
		go readArticles(wg, connNumber, retries)
		return
	} else {
		defer conn.Close()
	}

	for {

		article, ok := <-articlesChan
		if !ok {
			return
		}

		var (
			body io.Reader
			part *yenc.Part
		)

		articleCounter.Add(1)

		// read Article
		if body, err = read(conn, article.id); err != nil {
			Log.Debug("Error loading article with message id <%v>: %v", article.id, err)
			article.retries++
			if article.retries <= conf.Retries {
				failedArticlesChan <- article
			} else {
				Log.Warn("After %d retries unable to load article with message id <%v>: %v", article.retries-1, article.id, err)
				missingArticles.add(article.id)
				downloadProgressBar.Add(1)
				if missingArticles.len() > totalParts["par2"] {
					checkForFatalErr(fmt.Errorf("Number of missing articles too high!"))
				}
			}
			continue
		}
		// decode article body
		if part, err = yenc.Decode(body); err != nil {
			Log.Warn("Unable to decode body of the article with message id <%v>: %v", article.id, err)
			missingArticles.add(article.id)
			continue
		} else {
			totalBytesLoaded := totalBytesLoaded.Add(part.Size)
			totalPartsLoaded := totalPartsLoaded.Add(1)
			// estimate the total size to be downloaded based on the size of the first 10 articles and the total article count
			if downloadProgressBar != nil && totalPartsLoaded <= 10 {
				downloadProgressBar.ChangeMax64((totalBytesLoaded / totalPartsLoaded) * int64(totalParts[article.partType]))
			}
			fileWriters.runOnce(part.Name)
			fileChannels.channels[part.Name] <- part
		}

	}
}

func TryCatch(f func()) func() error {
	return func() (err error) {
		defer func() {
			if panicInfo := recover(); panicInfo != nil {
				err = fmt.Errorf("%v, %s", panicInfo, string(debug.Stack()))
				return
			}
		}()
		f() // calling the decorated function
		return err
	}
}
