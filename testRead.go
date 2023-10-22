package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Tensai75/nntp"
)

var (
	testReadCounter Counter
)

func (c *safeConn) testArticle(messageId string) (*nntp.Article, error) {
	// for reading error testing
	counter := testReadCounter.inc()
	if counter%150 == 0 {
		// return nil, fmt.Errorf("Test Error")
	}

	return readFromFile(messageId)
}

func read(conn *safeConn, messageId string) (*nntp.Article, error) {
	if conf.Test != "" {
		return conn.testArticle(messageId)
	}
	return conn.Article(fmt.Sprintf("<%v>", messageId))
}

func readFromFile(messageId string) (*nntp.Article, error) {

	var (
		readFile []byte
		article  nntp.Article
		body     bytes.Buffer
	)

	if readFile, err = os.ReadFile(filepath.Join(conf.Test, messageId+".txt")); err != nil {
		return nil, fmt.Errorf("Unable to load message <%v>", messageId)
	}
	expHeader, _ := regexp.Compile(`(Subject|Date|From|Message-ID|Path|Newsgroups): (.*)[ \n\r]`)
	expBody, _ := regexp.Compile("=ybegin[\\w\\W]+")

	headers := expHeader.FindAllStringSubmatch(string(readFile), -1)
	body.Write([]byte(expBody.FindString(string(readFile))))

	article.Header = make(map[string][]string)
	for _, header := range headers {
		article.Header[strings.Trim(header[1], " \r\n")] = append(article.Header[strings.Trim(header[1], " \r\n")], strings.Trim(header[2], " \r\n"))
	}
	article.Body = &body

	return &article, nil

}
