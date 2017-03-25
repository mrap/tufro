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

type RequestType int

const (
	RequestTypeTweet RequestType = iota
	RequestTypeDM
)

type Request struct {
	Type       RequestType
	Message    string
	Origin     *location.GeoPoint
	User       anaconda.User
	QueryFrom  string
	QueryTo    string
	From       *location.Location
	To         *location.Location
	Routes     []route.Route
	IsRetrying bool
}

var reLocStrings = regexp.MustCompile(`(?i)\A(?:@\w+\s+)*(\b.+\b)?(?:\s*->\s*|\s+to\s+)([[:^punct:],]+)\b`)

func ExtractLocationStrings(text string) (string, string) {
	matches := reLocStrings.FindStringSubmatch(html.UnescapeString(text))
	switch len(matches) {
	case 2:
		return "", matches[1]
	case 3:
		return matches[1], matches[2]
	default:
		log.Println("Could not parse two locations from text %s", text)
		return "", ""
	}
}

func (req *Request) Populate() error {
	req.QueryFrom, req.QueryTo = ExtractLocationStrings(req.Message)

	// Use tweet's location as from location if none given
	if req.QueryFrom == "" && req.Origin != nil {
		req.From = &location.Location{
			Coordinates: req.Origin,
		}
	} else {
		req.From = location.SearchTopLocation(req.QueryFrom, req.Origin)
		if req.From == nil {
			return fmt.Errorf("Unable to find [from] location: '%s'\n", req.QueryFrom)
		}

		// Use from as tweet origin if none provided
		if req.Origin == nil {
			req.From.PopulateCoordinates()
			req.Origin = req.From.Coordinates
		}
	}

	req.To = location.SearchTopLocation(req.QueryTo, req.Origin)
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

func (req *Request) ToFromText() string {
	buf := bytes.Buffer{}
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
	return req.ToFromText() + " " + msg
}

func (req *Request) ResponseText() (string, error) {
	if len(req.Routes) == 0 {
		return "", fmt.Errorf("Can't form response text without any routes!")
	}
	tripTime := duration(req.optimalRoute().TotalTime())
	tripTimeRT := duration(req.optimalRoute().TotalTimeRT())
	return fmt.Sprintf(
		"@%s %s -> %s right now: %.0f mins. (Usually %.0f mins)",
		req.User.ScreenName,
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
