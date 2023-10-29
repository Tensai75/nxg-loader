package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
)

var (
	testReadCounter atomic.Int64
)

func (c *safeConn) testBody(messageId string) (io.Reader, error) {
	// for reading error testing
	counter := testReadCounter.Add(1)
	if counter%150 == 0 {
		// return nil, fmt.Errorf("Test Error")
	}

	return readFromFile(messageId)
}

func read(conn *safeConn, messageId string) (io.Reader, error) {
	if conf.Test != "" {
		return conn.testBody(messageId)
	}
	return conn.Body(fmt.Sprintf("<%v>", messageId))
}

func readFromFile(messageId string) (io.Reader, error) {

	var (
		readFile []byte
		body     bytes.Buffer
	)

	if readFile, err = os.ReadFile(filepath.Join(conf.Test, messageId+".txt")); err != nil {
		return nil, fmt.Errorf("Unable to load message <%v>", messageId)
	}
	expBody, _ := regexp.Compile("=ybegin[\\w\\W]+")
	body.Write([]byte(expBody.FindString(string(readFile))))

	return &body, nil

}
