package twitter

import (
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/mrap/waze/location"
	"github.com/mrap/waze/route"
)

type Request struct {
	Tweet     *anaconda.Tweet
	QueryFrom string
	QueryTo   string
	From      *location.Location
	To        *location.Location
	Routes    []route.Route
}

var reLocStrings = regexp.MustCompile(`(?i)\A@\w+\s+\b(.+)\b(?:\s*->\s*|\s+to\s+)([[:^punct:]]+)\b`)

func ExtractLocationStrings(tweet *anaconda.Tweet) (string, string) {
	matches := reLocStrings.FindStringSubmatch(tweet.Text)
	if len(matches) != 3 {
		log.Println("Could not parse two locations from tweet %s", tweet.Text)
		return "", ""
	} else {
		return matches[1], matches[2]
	}
}

func (req *Request) Populate() error {
	req.QueryFrom, req.QueryTo = ExtractLocationStrings(req.Tweet)

	tweetOrigin := TweetGeoPoint(req.Tweet)
	req.From = location.SearchTopLocation(req.QueryFrom, tweetOrigin)
	if req.From == nil {
		return fmt.Errorf("Unable to find [from] location: '%s'\n", req.QueryFrom)
	}

	if tweetOrigin == nil {
		req.From.PopulateCoordinates()
		tweetOrigin = req.From.Coordinates
	}
	req.To = location.SearchTopLocation(req.QueryTo, tweetOrigin)
	if req.To == nil {
		return fmt.Errorf("Unable to find [to] location: '%s'\n", req.QueryTo)
	}

	var err error
	req.Routes, err = route.GetRoutes(req.From, req.To)
	if err != nil || len(req.Routes) == 0 {
		return fmt.Errorf("Unable to find routes from '%s' to '%s'. Error: %s", req.QueryFrom, req.QueryTo, err)
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

func (req *Request) ResponseText() (string, error) {
	if len(req.Routes) == 0 {
		return "", fmt.Errorf("Can't form response text without any routes!")
	}
	tripTime := time.Duration(req.Routes[0].TotalTime()) * time.Second
	tripTimeRT := time.Duration(req.Routes[0].TotalTimeRT()) * time.Second
	return fmt.Sprintf(
		"@%s %s -> %s right now: %.0f mins. (Usually %.0f mins)",
		req.Tweet.User.ScreenName,
		req.QueryFrom,
		req.QueryTo,
		tripTimeRT.Minutes(),
		tripTime.Minutes()), nil
}
