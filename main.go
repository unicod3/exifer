package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"log"
	"net/http"
	"os/exec"
)

type TagXML struct {
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
}

type TableXML struct {
	Text string `xml:",chardata"`
	Name string `xml:"name,attr"`
	G0   string `xml:"g0,attr"`
	G1   string `xml:"g1,attr"`
	G2   string `xml:"g2,attr"`
	Desc struct {
		Text string `xml:",chardata"`
		Lang string `xml:"lang,attr"`
	} `xml:"desc"`
	Tag []TagXML `xml:"tag"`
}

func main() {
	log.Println("Please visit:  http://localhost:3333/tags")
	// handle the endpoint
	http.HandleFunc("/tags", handleTags)
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

var (
	xmlData  chan string
	jsonData chan Response
	quit     chan int
)

func handleXMLStream(ctx context.Context) {
	select {
	case data := <-xmlData:

		// response struct
		var resp Response

		// initiate buffer and Decoder for streaming
		b := bytes.NewBufferString(data)
		d := xml.NewDecoder(b)
		for {

			// XML token in the stream
			t, err := d.Token()

			if err != nil {
				break
			}

			// get an instance

			switch et := t.(type) {

			case xml.StartElement:
				// get the desired element when it starts
				if et.Name.Local == "table" {
					tableXml := &TableXML{}

					// Decode it
					if err := d.DecodeElement(&tableXml, &et); err != nil {
						log.Fatal(err)
					}

					// Populate the data
					for _, tagXml := range tableXml.Tag {
						var tag Tag
						tag.Path = tagXml.Name
						tag.Writable = tagXml.Writable
						tag.Path = tableXml.Name + ":" + tagXml.Name
						tag.Group = tableXml.Name
						tag.Type = tagXml.Type

						for _, val := range tagXml.Desc {
							d := map[string]string{
								val.Lang: val.Text,
							}
							tag.Description = append(tag.Description, d)
						}
						resp.Tags = append(resp.Tags, tag)
					}
				} else if et.Name.Local == "taginfo" {
					log.Println("XML Decode Stream begins")
				}

			case xml.EndElement:

				if et.Name.Local != "taginfo" {
					continue
				}
				if et.Name.Local != "table" {
					continue
				}
			}
		}
		// push some bits
		jsonData <- resp
	case <-ctx.Done():
		quit <- 1
	}
}

func handleTags(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// initiate the chanels
	xmlData = make(chan string)
	jsonData = make(chan Response)
	quit = make(chan int)

	// handle the xml
	go handleXMLStream(ctx)

	cmd := exec.Command("exiftool", "-listx")
	_, err := cmd.StdinPipe()
	if err != nil {
		w.Write([]byte("Something went wrong"))
		return
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	xmlData <- out.String()

	select {
	case d := <-jsonData:
		// Wait for decoded data
		log.Println("JSON Encode stream begins")
		flusher, ok := w.(http.Flusher)
		if !ok {
			panic("expected http.ResponseWriter to be an http.Flusher")
		}

		// Encode data and push some bits into screen
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		if err := enc.Encode(&d); err != nil {
			log.Println(err)
		}
		flusher.Flush()
		log.Println("Operation Completed")
	case <-ctx.Done():
		cmd.Process.Kill()
		log.Println("Command Killed!")
	case <-quit:
		cmd.Process.Kill()
		log.Println("Command Killed")
	}
}
