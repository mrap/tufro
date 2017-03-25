package main

import (
	"bytes"
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
			switch item := elem.(type) {
			case anaconda.Tweet:
				if item.InReplyToUserID == mainUser.Id {
					requestQueue <- &Request{
						Type:    RequestTypeTweet,
						Origin:  TweetGeoPoint(&item),
						Message: item.Text,
						User:    item.User,
					}
				}
			case anaconda.DirectMessage:
				if item.RecipientId == mainUser.Id {
					requestQueue <- &Request{
						Type:    RequestTypeDM,
						Message: item.Text,
						User:    item.Sender,
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
		msg := "Sorry: " + err.Error()
		reply(req, msg)
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
		reply(req, msg)
	} else {
		if !req.IsRetrying {
			req.IsRetrying = true

			msg := req.MessageText(fmt.Sprintf("WAIT. %.0fm of traffic (%.0fm total). I'll notify you when clear.", trafficDuration.Minutes(), req.RouteRT().Minutes()))
			reply(req, msg)
		}

		log.Printf("Route has traffic delay of %f mins. Will retry.\n", trafficDuration.Minutes())
		reprocessAfter(req, defaultReprocessDelay)
	}
}

func reply(req *Request, msg string) {
	if req.Type == RequestTypeTweet {
		postTweet(req.User, msg)
	} else {
		sendDM(req.User, msg)
	}
}

func postTweet(to anaconda.User, text string) {
	_, err := Api.PostTweet(ReplyTweetMessage(to, text), url.Values{})
	if err != nil {
		log.Println("Problem posting tweet", err)
	}
}

func sendDM(to anaconda.User, text string) {
	_, err := Api.PostDMToUserId(text, to.Id)
	if err != nil {
		log.Println("Problem sending dm", err)
	}
}

func ReplyTweetMessage(user anaconda.User, text string) string {
	buf := bytes.Buffer{}
	buf.WriteRune('@')
	buf.WriteString(user.ScreenName)
	buf.WriteRune(' ')
	buf.WriteString(text)
	return buf.String()
}

func reprocessAfter(req *Request, delay time.Duration) {
	go func() {
		select {
		case <-time.After(delay):
			requestQueue <- req
		}
	}()
}
