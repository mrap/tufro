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
	userRequests  = make(UserRequests)
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
					req := &Request{
						Type:    RequestTypeTweet,
						Origin:  TweetGeoPoint(&item),
						Message: item.Text,
						User:    item.User,
					}
					userRequests.Add(req)
					requestQueue <- req
				}
			case anaconda.DirectMessage:
				if item.RecipientId == mainUser.Id {
					req := &Request{
						Type:    RequestTypeDM,
						Message: item.Text,
						User:    item.Sender,
					}
					userRequests.Add(req)
					requestQueue <- req
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
	if req.IsCancelled {
		return
	}

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
	if req.IsCancelled {
		return
	}

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
		req.CancelRetry = make(chan bool)
		timer := time.NewTimer(delay)

		defer func() {
			close(req.CancelRetry)
			req.CancelRetry = nil
		}()

		select {
		case <-req.CancelRetry:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			requestQueue <- req
			return
		}
	}()
}
