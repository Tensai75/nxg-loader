package main

import (
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	"github.com/chrisfarms/yenc"
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
			body io.Reader
			part *yenc.Part
		)

		partsCounter.inc()

		// read Article
		if body, err = read(conn, message.id); err != nil {
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

		// decode article body
		if part, err = yenc.Decode(body); err != nil {
			Log.Warn("Unable to decode body of message id <%v>: %v", message.id, err)
			missingParts.add(message.id)
			continue
		} else {
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
