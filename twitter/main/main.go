package main

import (
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/ChimeraCoder/anaconda"
	. "github.com/mrap/goutil/builtin"
	. "github.com/mrap/tufro/twitter"
	"github.com/mrap/twitterget/api"
	"github.com/mrap/twitterget/streaming"
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
	defer close(s.C)

	for {
		select {
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
		msg := req.ReplyPrefix() + "Sorry: " + err.Error()
		postTweet(msg)
		return
	}

	responseQueue <- req
}

const defaultReprocessDelay = 2 * time.Minute
const maxTrafficDuration = 5 * time.Minute

func RespondToRequest(req *Request) {
	trafficDuration := req.TrafficDuration()

	if trafficDuration <= maxTrafficDuration {
		msg := req.MessageText(fmt.Sprintf("GO! %.0fm drive without traffic.", req.RouteRT().Minutes()))
		postTweet(msg)
	} else {
		if !req.IsRetrying {
			req.IsRetrying = true

			msg := req.MessageText(fmt.Sprintf("WAIT. %.0fm of traffic (%.0fm total). I'll tweet you when clear.", trafficDuration.Minutes(), req.RouteRT().Minutes()))
			postTweet(msg)
		}

		log.Printf("Route has traffic delay of %f mins. Will retry.\n", trafficDuration.Minutes())
		reprocessAfter(req, defaultReprocessDelay)
	}
}

func postTweet(text string) {
	_, err := Api.PostTweet(text, url.Values{})
	if err != nil {
		log.Println("Problem posting tweet", err)
	}
}

func reprocessAfter(req *Request, delay time.Duration) {
	go func() {
		select {
		case <-time.After(delay):
			requestQueue <- req
		}
	}()
}
