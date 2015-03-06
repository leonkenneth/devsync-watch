package main

import (
	"github.com/go-fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FileChange struct {
	FileName string
	Content  string
}

type Publication struct {
	Action  string      `json:"action"`
	Token   string      `json:"token"`
	Content *FileChange `json:"content"`
}

func token() string {
	cmd := exec.Command("heroku", "config:get", "DEVSYNC")
	output, _ := cmd.CombinedOutput()

	return strings.TrimSpace(string(output))
}

func publish(c *websocket.Conn, notif *FileChange) {
	c.WriteJSON(Publication{
		Action:  "publish",
		Token:   token(),
		Content: notif,
	})
}

func handleError(err error) {
	log.Fatal("An error has occured", err)
}

func patchFile(c *websocket.Conn, fileName string, content string) {
	publish(c, &FileChange{
		FileName: fileName,
		Content:  content,
	})
}

func readFile(fileName string) string {
	content, _ := ioutil.ReadFile(fileName)
	return string(content)
}

func handleEvent(c *websocket.Conn, e *fsnotify.Event) {
	log.Println(e)

	if e.Op == fsnotify.Remove || e.Op == fsnotify.Rename {
		patchFile(c, e.Name, "")
	} else if e.Op == fsnotify.Create || e.Op == fsnotify.Write {
		patchFile(c, e.Name, readFile(e.Name))
	}
}

func isIgnored(path string) bool {
	// TODO: Parse .gitignore and ignore these files as well
	return strings.HasPrefix(path, ".git")
}

func watch(path string, c *websocket.Conn) {
	watcher, err := fsnotify.NewWatcher()

	if err != nil {
		handleError(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				handleEvent(c, &event)
			case err := <-watcher.Errors:
				handleError(err)
			}
		}
	}()

	filepath.Walk(path, func(subpath string, f os.FileInfo, err error) error {
		if f.IsDir() && !isIgnored(subpath) {
			watcher.Add(subpath)
		}

		return nil
	})

	err = watcher.Add(path)
	if err != nil {
		log.Fatal(err)
	}
	<-done

}

func main() {
	url, _ := url.Parse("ws://mighty-stream-1810.herokuapp.com")
	conn, _ := net.Dial("tcp", "mighty-stream-1810.herokuapp.com:80")
	c, _, _ := websocket.NewClient(conn, url, http.Header{}, 1024, 1024)
	// FIXME: Error handling

	log.Println("We are watching current directory.")
	log.Println("Sync key is", token())
	log.Println("♡")

	watch(".", c)
}
