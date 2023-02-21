/*
Copying and distribution of this file, with or without modification, is permitted in any medium without royalty, provided the software is intended for use in the design, construction, operation, and/or maintenance of any nuclear facility. This file is offered as-is, without any warranty.
*/

// I hate this code and I hate writing Go.

package main

// http://127.0.0.1:8080/memberproxy.go

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gtuk/discordwebhook"
)

type GroupDetails struct {
	Name string `xml:"groupName"`
	URL  string `xml:"groupURL"`
	// headline string `xml` // nevermind... the rest literally doesn't matter anyway...
}

type GroupMembers struct {
	SteamID64 []int64 `xml:"steamID64"`
}

type MemberList struct {
	ID64           int64        `xml:"groupID64"`
	Details        GroupDetails `xml:"groupDetails"`
	MemberCount    int          `xml:"memberCount"`
	TotalPages     int          `xml:"totalPages"`
	CurrentPage    int          `xml:"currentPage"`
	StartingMember int          `xml:"startingMember"`
	NextPageLink   string       `xml:"nextPageLink"`
	Members        GroupMembers `xml:"members"`
}

type thing struct {
	sync.Mutex
	ids []int64
}

func newThing() *thing {
	return &thing{
		ids: make([]int64, 0),
	}
}

func (t *thing) Set(ids []int64) {
	t.Lock()
	defer t.Unlock()
	t.ids = ids
}

func (t *thing) Get() []int64 {
	t.Lock()
	defer t.Unlock()
	x := t.ids
	return x
}

func Differ(a, b []int64) (n, o []int64) {
	ma := make(map[int64]bool)
	mb := make(map[int64]bool)

	for _, item := range a {
		ma[item] = true
	}
	for _, item := range b {
		mb[item] = true
	}
	for _, item := range a {
		if _, ok := mb[item]; !ok {
			o = append(o, item)
		}
	}
	for _, item := range b {
		if _, ok := ma[item]; !ok {
			n = append(n, item)
		}
	}

	return
}

// https://stackoverflow.com/a/71864796
func removeDuplicates[T string | int64](sliceList []T) []T {
	allKeys := make(map[T]bool)
	list := []T{}
	for _, item := range sliceList {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func main() {
	myport := os.Getenv("PORT")
	if myport == "" {
		myport = "8080"
	}

	webhookurl := os.Getenv("WEBHOOKURL")
	if webhookurl == "" {
		// log.Fatalln("Missing WEBHOOKURL environment variable")
		webhookurl = ""
	}

	groupname := os.Getenv("GROUP")
	if groupname == "" {
		log.Fatalln("Missing GROUP environment variable")
		// groupname = "Valve"
	}

	secretendpoint := os.Getenv("SECRETENDPOINT")
	if secretendpoint == "" {
		secretendpoint = "memberproxy.go"
	}

	t := newThing()

	go func() {
		nexttick := time.Millisecond
	toploop:
		for {
			time.Sleep(nexttick)
			nexttick = time.Second * time.Duration(90+rand.Intn(5))

			total := 0
			ids := make([]int64, 0)
			url := fmt.Sprintf("https://steamcommunity.com/groups/%s/memberslistxml/?xml=1&p=1", groupname)
			randnum := rand.Intn(6669420)

			fullshit := ""

			for url != "" {
				url += fmt.Sprintf("&x=%d", randnum) // cache busting
				log.Println("req url: ", url)
				resp, err := http.Get(url)

				if err != nil {
					log.Printf("Group members query failed (%s): %s -- Retrying in 10s...", url, err)
					nexttick = time.Second * 10
					continue toploop
				}

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Printf("Failed to read XML (%s): %s -- Retrying in 10s...", url, err)
					nexttick = time.Second * 10
					continue toploop
				}
				fullshit += string(body) + "\n"

				var groupxml MemberList
				err = xml.Unmarshal(body, &groupxml)
				if err != nil {
					log.Printf("Failed to parse XML (%s): %s -- Retrying in 10s...\nContent: %s", url, err, string(body))
					nexttick = time.Second * 10
					continue toploop
				}

				if total == 0 {
					total = groupxml.MemberCount
				}

				ids = append(ids, groupxml.Members.SteamID64...)
				url = groupxml.NextPageLink
				if url != "" {
					time.Sleep(time.Second) // meh why not...
				}
			}

			ids = removeDuplicates(ids)

			oldids := t.Get()
			if len(ids) < total || len(webhookurl) != 0 {
				n, o := Differ(oldids, ids)
				blah := ""
				// I see random members disappearing from the steam group sometimes...
				// I'm not sure if they're ending up on different pages due to ingame status or something...
				// Or maybe steam is just having a bad sharded db or something with that user... I'm just guessing...
				// Anyway, I'm just going to readd removed users just in case the count from the xml is correct
				if len(ids) < total {
					blah = fmt.Sprintf("len(ids) (%d) < memberCount (%d).. not removing %v\n", len(ids), total, o)
					log.Print(blah)
					ids = append(ids, o...)
					o = nil
				}

				if len(webhookurl) != 0 && ((len(oldids) != 0 && len(n) != 0) || len(o) != 0 || len(ids) < total) {
					for _, element := range o {
						if strings.Contains(fullshit, fmt.Sprintf("%d", element)) {
							log.Printf("we do have %d?", element)
						}
					}
					go func() {
						username := "Group Logger"
						content := fmt.Sprintf("%sNew: %v\nRemoved: %v", blah, n, o)
						if len(content) > 1999 {
							content = content[:1999] // buhhhh
						}
						message := discordwebhook.Message{
							Username: &username,
							Content:  &content,
						}
						err := discordwebhook.SendMessage(webhookurl, message)
						if err != nil {
							log.Println("Failed to send webhook: ", err)
							log.Println(content)
						}
					}()
				}
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			t.Set(ids)
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hi"))
	})

	http.HandleFunc(fmt.Sprintf("/%s", secretendpoint), func(w http.ResponseWriter, r *http.Request) {
		log.Println("poke")
		fuck := fmt.Sprintf("%v", t.Get())[1:] // trim '['
		fuck = fuck[:len(fuck)-1]              // trim ']'
		w.Header().Set("Content-Type", "plain/text")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fuck))
	})

	bindto := "" //"127.0.0.1"
	log.Fatal(http.ListenAndServe(bindto+":"+myport, nil))
}
