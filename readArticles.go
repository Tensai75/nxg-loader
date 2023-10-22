package main

import (
	"bytes"
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	"github.com/Tensai75/nntp"
)

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
			failed := failedConnections.inc()
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

		message, ok := <-messagesChan
		if !ok {
			return
		}

		var (
			article   *nntp.Article
			bodyBytes []byte
		)

		partsCounter.inc()

		if article, err = read(conn, message.id); err != nil {
			Log.Debug("%v", message.id, err)
			message.retries++
			if message.retries <= conf.Retries {
				if err := TryCatch(func() { messagesChan <- message })(); err != nil {
					Log.Debug("Error while trying to add message id <%v> back to the queue: %v", message.id, err)
					missingParts.add(message.id)
				} else {
					Log.Debug("Added message id <%v> back to reading queue", message.id)
				}
			} else {
				Log.Warn("After %d retries unable to read message id <%v>: %v", message.retries-1, message.id, err)
				missingParts.add(message.id)
			}
			continue
		}

		if bodyBytes, err = io.ReadAll(article.Body); err != nil {
			Log.Error("Unable to read body of message id <%v>: %v", message.id, err)
			missingParts.add(message.id)
			continue
		}
		article.Body = bytes.NewReader(bodyBytes)

		articlesChan <- *article

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
