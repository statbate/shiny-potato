package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("./test room server proxy")
		return
	}

	room := os.Args[1]
	server := os.Args[2]
	proxy := os.Args[3]

	u, token, id := getToken(room)

	startRoom(room, server, proxy, u, token, id)
}

func getToken(room string) (*url.URL, string, string) {
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, "https://ru.stripchat.com/"+room, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:33.0) Gecko/20100101 Firefox/33.0")
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Add("Accept-Language", "en-US,en;q=0.5")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Referer", "https://ru.stripchat.com")
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	} else if rsp.StatusCode != http.StatusOK {
		panic("failed to get rsp")
	}

	defer rsp.Body.Close()

	buf, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		panic(err)
	}
	re := regexp.MustCompile(`"websocketUrl":"*(.*?)\s*"`)
	m := re.FindSubmatch(buf)
	if len(m) != 2 {
		panic("not match")
	}
	r := bytes.ReplaceAll(m[1], []byte(`\u002F`), []byte(`/`))
	u, err := url.Parse(string(r))
	if err != nil {
		panic(err)
	}

	re = regexp.MustCompile(`"token":"*(.*?)\s*"`)
	m = re.FindSubmatch(buf)
	if len(m) != 2 {
		panic("not match")
	}

	re = regexp.MustCompile(`img.strpst.com/thumbs/*(.*?)\s*"`)
	id := re.FindSubmatch(buf)
	if len(id) != 2 {
		panic("not match")
	}
	xid := string(id[1])
	return u, string(m[1]), xid[strings.Index(xid, "/")+1:]
}
