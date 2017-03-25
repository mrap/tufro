package twitter

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"regexp"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/mrap/waze/location"
	"github.com/mrap/waze/route"
)

type Request struct {
	Tweet      *anaconda.Tweet
	QueryFrom  string
	QueryTo    string
	From       *location.Location
	To         *location.Location
	Routes     []route.Route
	IsRetrying bool
}

var reLocStrings = regexp.MustCompile(`(?i)\A@\w+\s+(\b.+\b)?(?:\s*->\s*|\s+to\s+)([[:^punct:],]+)\b`)

func ExtractLocationStrings(tweet *anaconda.Tweet) (string, string) {
	matches := reLocStrings.FindStringSubmatch(html.UnescapeString(tweet.Text))
	switch len(matches) {
	case 2:
		return "", matches[1]
	case 3:
		return matches[1], matches[2]
	default:
		log.Println("Could not parse two locations from tweet %s", tweet.Text)
		return "", ""
	}
}

func (req *Request) Populate() error {
	req.QueryFrom, req.QueryTo = ExtractLocationStrings(req.Tweet)
	tweetOrigin := TweetGeoPoint(req.Tweet)

	// Use tweet's location as from location if none given
	if req.QueryFrom == "" && tweetOrigin != nil {
		req.From = &location.Location{
			Coordinates: tweetOrigin,
		}
	} else {
		req.From = location.SearchTopLocation(req.QueryFrom, tweetOrigin)
		if req.From == nil {
			return fmt.Errorf("Unable to find [from] location: '%s'\n", req.QueryFrom)
		}

		// Use from as tweet origin if none provided
		if tweetOrigin == nil {
			req.From.PopulateCoordinates()
			tweetOrigin = req.From.Coordinates
		}
	}

	req.To = location.SearchTopLocation(req.QueryTo, tweetOrigin)
	if req.To == nil {
		return fmt.Errorf("Unable to find [to] location: '%s'\n", req.QueryTo)
	}

	var err error
	req.Routes, err = route.GetRoutes(req.From, req.To)
	if len(req.Routes) == 0 {
		return fmt.Errorf("Unable to find routes from '%s' to '%s'", req.QueryFrom, req.QueryTo)
	} else if err != nil {
		return fmt.Errorf("Unable to find routes from '%s' to '%s' with error: %s", req.QueryFrom, req.QueryTo, err.Error())
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
	} else if t.Place.BoundingBox.Type != "" {
		if bbCoords := t.Place.BoundingBox.Coordinates; len(bbCoords) > 0 {
			points := bbCoords[0]
			var longTotal float64
			var latTotal float64
			for _, c := range points {
				longTotal += c[0]
				latTotal += c[1]
			}

			totalCoords := float64(len(points))
			return &location.GeoPoint{
				Long: longTotal / totalCoords,
				Lat:  latTotal / totalCoords,
			}
		}
	}

	return nil
}

func (req *Request) ReplyPrefix() string {
	return "@" + req.Tweet.User.ScreenName + " "
}

func (req *Request) MessagePrefix() string {
	buf := bytes.Buffer{}
	buf.WriteRune('@')
	buf.WriteString(req.Tweet.User.ScreenName)
	buf.WriteRune(' ')

	if req.QueryFrom != "" {
		buf.WriteString(req.QueryFrom)
		buf.WriteRune(' ')
	}

	buf.WriteRune('-')
	buf.WriteRune('>')
	buf.WriteRune(' ')
	buf.WriteString(req.QueryTo)

	return buf.String()
}

func (req *Request) MessageText(msg string) string {
	return req.MessagePrefix() + " " + msg
}

func (req *Request) ResponseText() (string, error) {
	if len(req.Routes) == 0 {
		return "", fmt.Errorf("Can't form response text without any routes!")
	}
	tripTime := duration(req.optimalRoute().TotalTime())
	tripTimeRT := duration(req.optimalRoute().TotalTimeRT())
	return fmt.Sprintf(
		"@%s %s -> %s right now: %.0f mins. (Usually %.0f mins)",
		req.Tweet.User.ScreenName,
		req.QueryFrom,
		req.QueryTo,
		tripTimeRT.Minutes(),
		tripTime.Minutes()), nil
}

func (req *Request) optimalRoute() *route.Route {
	if len(req.Routes) == 0 {
		return nil
	} else {
		return &req.Routes[0]
	}
}

func (req *Request) RouteRT() time.Duration {
	return duration(req.optimalRoute().TotalTimeRT())
}

func (req *Request) TrafficDuration() time.Duration {
	tripTime := duration(req.optimalRoute().TotalTime())
	tripTimeRT := duration(req.optimalRoute().TotalTimeRT())
	return tripTimeRT - tripTime
}

func duration(secs route.Seconds) time.Duration {
	return time.Duration(secs) * time.Second
}
