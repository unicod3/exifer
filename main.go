package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os/exec"
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

func handleXML(xmlStr string) []byte {
	var taginfo Taginfo
	data := []byte(xmlStr)
	err := xml.Unmarshal(data, &taginfo)
	if err != nil {
		return []byte("")
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
			//fmt.Printf("%v", tag)
		}
	}

	//fmt.Printf("%#v\n\n", taginfo)
	b, err := json.Marshal(resp)
	if err != nil {
		fmt.Println(err)
		return []byte("")
	}
	return b
}

func handleTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
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

	res := handleXML(out.String())
	_, err = w.Write(res)
	if err != nil {
		w.Write([]byte("Something went wrong"))
		return
	}
}
