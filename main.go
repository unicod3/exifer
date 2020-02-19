package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
)

type Taginfo struct {
	XMLName xml.Name `xml:"taginfo"`
	Text    string   `xml:",chardata"`
	Table   []struct {
		Text string `xml:",chardata"`
		Name string `xml:"name,attr"`
		G0   string `xml:"g0,attr"`
		G1   string `xml:"g1,attr"`
		G2   string `xml:"g2,attr"`
		Desc struct {
			Text string `xml:",chardata"`
			Lang string `xml:"lang,attr"`
		} `xml:"desc"`
		Tag []struct {
			Text     string `xml:",chardata"`
			ID       string `xml:"id,attr"`
			Name     string `xml:"name,attr"`
			Type     string `xml:"type,attr"`
			Writable string `xml:"writable,attr"`
			G2       string `xml:"g2,attr"`
			Desc     []struct {
				Text string `xml:",chardata"`
				Lang string `xml:"lang,attr"`
			} `xml:"desc"`
		} `xml:"tag"`
	} `xml:"table"`
}

func main() {
	fmt.Println("Please visit:  http://localhost:3333/tags")
	// handle the endpoint
	http.HandleFunc("/tags", handleTagsV2)
	if err := http.ListenAndServe(":3333", nil); err != nil {
		panic(err)
	}
}

type Tag struct {
	Writable    string              `json:"writable"`
	Path        string              `json:"path"`
	Group       string              `json:"group"`
	Description []map[string]string `json:"description"`
	Type        string              `json:"type"`
}
type Response struct {
	Tags []Tag `json:"tags"`
}

func handeStream(w io.Writer, r io.Reader) ([]byte, error) {
	var out []byte
	buf := make([]byte, 1024, 1024)
	for {
		n, err := r.Read(buf[:])
		if n > 0 {
			d := buf[:n]
			out = append(out, d...)
			_, err := w.Write(d)
			if err != nil {
				return out, err
			}
		}
		if err != nil {
			// Read returns io.EOF at the end of file, which is not an error for us
			if err == io.EOF {
				err = nil
			}
			return out, err
		}
	}
}

func handleXML(xmlVal []byte, outch chan<- []byte) {
	xmlStr := string(xmlVal)
	var taginfo Taginfo
	data := []byte(xmlStr)
	err := xml.Unmarshal(data, &taginfo)
	if err != nil {
		fmt.Println(err)
	}

	var resp Response
	// generate result with the xml
	for _, table := range taginfo.Table {
		for _, tag := range table.Tag {
			var t Tag
			t.Writable = tag.Writable
			t.Path = table.Name + ":" + tag.Name
			t.Group = table.Name
			t.Type = tag.Type
			for _, desc := range tag.Desc {
				d := map[string]string{
					desc.Lang: desc.Text,
				}

				t.Description = append(t.Description, d)
			}
			resp.Tags = append(resp.Tags, t)
		}
	}

	b, err := json.Marshal(resp)
	if err != nil {
		fmt.Println(err)
	}
	outch <- b
}

/*
 * HTTP Endpoints
 */

/*
 * handle Tags V2 handles the cmd output stream and
 */
func handleTagsV2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	var wg sync.WaitGroup
	res := make(chan []byte)
	cmd := exec.Command("exiftool", "-listx")

	var stdout []byte
	var errStdout, errStderr error
	stdoutIn, _ := cmd.StdoutPipe()
	err := cmd.Start()
	if err != nil {
		log.Fatalf("cmd.Start() failed with '%s'\n", err)
	}

	// use wait group to handle stream data
	wg.Add(1)
	go func(cctx context.Context) {
		//defer wg.Done()
		//d := make(chan []byte)
		stdout, errStdout = handeStream(os.Stdout, stdoutIn)
		wg.Done()
		select {
		case <-cctx.Done():
			if err := cmd.Process.Kill(); err != nil {
				log.Fatal("failed to kill process: ", err)
			}
			log.Println("Process Killed")
			return
		}
	}(ctx)
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	if errStdout != nil || errStderr != nil {
		log.Fatal("failed to capture stdout or stderr\n")
	}

	// fire a goroutine
	go handleXML(stdout, res)

	// wait for the result and pass the stream to browser
	_, err = w.Write(<-res)
	if err != nil {
		w.Write([]byte("Something went wrong"))
	}
}
