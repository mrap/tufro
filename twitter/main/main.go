package main

import (
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/ChimeraCoder/anaconda"
	. "github.com/mrap/tufro/twitter"
	"github.com/mrap/twitterget/api"
	"github.com/mrap/twitterget/streaming"
	. "github.com/mrap/util/builtin"
)

var (
	Api           *anaconda.TwitterApi
	requestQueue  chan (*Request)
	responseQueue chan (*Request)
)

func main() {
	Api = api.NewApi(*api.LoadAuthConfig("secrets"))
	mainUser, err := Api.GetSelf(url.Values{})
	PanicIf(err)

	go ListenForRequests()

	s := streaming.StartUserStream(Api)
	for {
		select {
		case <-s.Quit:
			log.Println("Closing user stream")
			return
		case elem := <-s.C:
			if tweet, ok := elem.(anaconda.Tweet); ok {
				if tweet.InReplyToUserID == mainUser.Id {
					requestQueue <- &Request{
						Tweet: &tweet,
					}
				}
			}
		}
	}
}

func ListenForRequests() {
	requestQueue = make(chan *Request)
	responseQueue = make(chan *Request)
	for {
		select {
		case req := <-requestQueue:
			go ProcessNewRequest(req)
		case req := <-responseQueue:
			go RespondToRequest(req)
		}
	}
}

func ProcessNewRequest(req *Request) {
	err := req.Populate()
	if err != nil {
		log.Println(err)
		return
	}
	responseQueue <- req
}

func RespondToRequest(req *Request) {
	tripTime := time.Duration(req.Routes[0].TotalTime()) * time.Second
	tripTimeRT := time.Duration(req.Routes[0].TotalTimeRT()) * time.Second
	status := fmt.Sprintf(
		"@%s %s -> %s right now: %.0f mins. (Usually %.0f mins)",
		req.Tweet.User.ScreenName,
		req.QueryFrom,
		req.QueryTo,
		tripTimeRT.Minutes(),
		tripTime.Minutes())

	_, err := Api.PostTweet(status, url.Values{})
	if err != nil {
		log.Println("Problem posting tweet", err)
	}
}
