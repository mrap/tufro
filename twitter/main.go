package main

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/mrap/twitterget/api"
	"github.com/mrap/twitterget/streaming"
	. "github.com/mrap/util/builtin"
	"github.com/mrap/waze/location"
	"github.com/mrap/waze/route"
)

type Request struct {
	Tweet  *anaconda.Tweet
	From   *location.Location
	To     *location.Location
	Routes []route.Route
}

var (
	Api               *anaconda.TwitterApi
	mainUser          anaconda.User
	requestQueue      chan (*Request)
	responseQueue     chan (*Request)
	parseLocationsExp *regexp.Regexp
)

func init() {
	parseLocationsExp = regexp.MustCompile(`^@\S+\s+(.+)\s*(?:->|-&gt;)\s*([^?]+).*$`)
}

func ParseLocationStrings(tweet *anaconda.Tweet) (string, string) {
	matches := parseLocationsExp.FindStringSubmatch((tweet.Text))
	if len(matches) != 3 {
		log.Println("Could not parse two locations from tweet %s", tweet.Text)
		return "", ""
	} else {
		return matches[1], matches[2]
	}
}

func main() {
	var err error

	Api = api.NewApi(*api.LoadAuthConfig("secrets"))
	mainUser, err = Api.GetSelf(url.Values{})
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
	d := time.Duration(req.Routes[0].TotalTime()) * time.Second
	status := fmt.Sprintf(
		"@%s Driving %s -> %s will take you about %.2f minutes",
		req.Tweet.User.ScreenName,
		req.From.Description,
		req.To.Description,
		d.Minutes())

	_, err := Api.PostTweet(status, url.Values{})
	if err != nil {
		log.Println("Problem posting tweet", err)
	}
}

func (req *Request) Populate() error {
	fromStr, toStr := ParseLocationStrings(req.Tweet)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		req.From = location.SearchTopLocation(fromStr)
	}()
	go func() {
		defer wg.Done()
		req.To = location.SearchTopLocation(toStr)
	}()
	wg.Wait()

	if req.From == nil {
		return fmt.Errorf("Unable to find [from] location: '%s'\n", fromStr)
	}
	if req.To == nil {
		return fmt.Errorf("Unable to find [to] location: '%s'\n", toStr)
	}

	var err error
	wg.Add(1)
	go func() {
		defer wg.Done()
		req.Routes, err = route.GetRoutes(req.From, req.To)
	}()
	wg.Wait()

	if err != nil || len(req.Routes) == 0 {
		return fmt.Errorf("Unable to find routes from '%s' to '%s'", fromStr, toStr)
	}

	return nil
}
