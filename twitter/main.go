package main

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
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
	parseLocationsExp = regexp.MustCompile(`^@\S+\s+(.+)\s*(?:->|-&gt;|to)\s*([^?]+).*$`)
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
	tripTime := time.Duration(req.Routes[0].TotalTime()) * time.Second
	tripTimeRT := time.Duration(req.Routes[0].TotalTimeRT()) * time.Second
	status := fmt.Sprintf(
		"@%s %s -> %s right now: %.0f mins. (Usually %.0f mins)",
		req.Tweet.User.ScreenName,
		req.From,
		req.To,
		tripTimeRT.Minutes(),
		tripTime.Minutes())

	_, err := Api.PostTweet(status, url.Values{})
	if err != nil {
		log.Println("Problem posting tweet", err)
	}
}

func (req *Request) Populate() error {
	fromStr, toStr := ParseLocationStrings(req.Tweet)

	tweetOrigin := TweetGeoPoint(req.Tweet)
	req.From = location.SearchTopLocation(fromStr, tweetOrigin)
	if req.From == nil {
		return fmt.Errorf("Unable to find [from] location: '%s'\n", fromStr)
	}

	if tweetOrigin == nil {
		req.From.PopulateCoordinates()
		tweetOrigin = req.From.Coordinates
	}
	req.To = location.SearchTopLocation(toStr, tweetOrigin)
	if req.To == nil {
		return fmt.Errorf("Unable to find [to] location: '%s'\n", toStr)
	}

	var err error
	req.Routes, err = route.GetRoutes(req.From, req.To)
	if err != nil || len(req.Routes) == 0 {
		return fmt.Errorf("Unable to find routes from '%s' to '%s'. Error: %s", fromStr, toStr, err)
	}

	return nil
}

func TweetGeoPoint(t *anaconda.Tweet) *location.GeoPoint {
	if t.HasCoordinates() {
		long, _ := t.Longitude()
		lat, _ := t.Latitude()
		return &location.GeoPoint{
			Long: long,
			Lat:  lat,
		}
	} else {
		return nil
	}
}
