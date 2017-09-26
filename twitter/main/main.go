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

func main() {
	s := NewServer()
	s.start()
}

type Server struct {
	api           *anaconda.TwitterApi
	mainUser      anaconda.User
	requestQueue  chan (*Request)
	responseQueue chan (*Request)
	userRequests  UserRequests
}

func NewServer() *Server {
	s := &Server{
		requestQueue:  make(chan *Request),
		responseQueue: make(chan *Request),
		userRequests:  make(UserRequests),
	}

	err := s.refreshApi()
	PanicIf(err)

	return s
}

func (s *Server) refreshApi() (err error) {
	if s.api != nil {
		s.api.Close()
		s.api = nil
	}

	s.api = api.NewApi(*api.LoadAuthConfig("secrets"))
	s.mainUser, err = s.api.GetSelf(url.Values{})

	return err
}

func (s *Server) start() {
	go s.ListenForRequests()

	for {
		s.listenForTwitterEvents()
		err := s.refreshApi()
		PanicIf(err)
	}
}

func (s *Server) listenForTwitterEvents() {
	stream := streaming.StartUserStream(s.api)
	defer stream.Stop()

	for elem := range stream.C {
		switch item := elem.(type) {
		case anaconda.Tweet:
			if item.InReplyToUserID == s.mainUser.Id {
				req := NewRequest(RequestTypeTweet, item.Text, item.User)
				req.Origin = TweetGeoPoint(&item)
				s.enqueueRequest(req)
			}
		case anaconda.DirectMessage:
			if item.RecipientId == s.mainUser.Id {
				req := NewRequest(RequestTypeDM, item.Text, item.Sender)
				s.enqueueRequest(req)
			}
		}
	}

	log.Println("listenForTwitterEvents stream closed")
}

func (s *Server) ListenForRequests() {
	for {
		select {
		case req := <-s.requestQueue:
			go s.ProcessNewRequest(req)
		case req := <-s.responseQueue:
			go s.RespondToRequest(req)
		}
	}
}

func logRequest(req *Request) {
	var requestType string
	switch req.Type {
	case RequestTypeTweet:
		requestType = "T"
	case RequestTypeDM:
		requestType = "DM"
	}

	originString := ""
	if req.Origin != nil {
		originString = fmt.Sprintf("x=%s y=%s", req.Origin.LongString(), req.Origin.LatString())
	}

	log.Printf("REQUEST(%s) from %s:\n%s\nOrigin: %s", requestType, req.User.ScreenName, req.Message, originString)
}

func (s *Server) enqueueRequest(req *Request) {
	logRequest(req)
	s.userRequests.Add(req)
	s.requestQueue <- req
}

func (s *Server) ProcessNewRequest(req *Request) {
	if req.IsCancelled {
		return
	}

	err := req.Populate()
	if err != nil {
		log.Println(err)
		msg := "Sorry: " + err.Error()
		s.reply(req, msg)
		return
	}

	s.responseQueue <- req
}

const defaultReprocessDelay = 2 * time.Minute
const maxTrafficDelayFactor = 1.33
const busyTrafficDelayFactor = 1 + ((maxTrafficDelayFactor - 1) / 2)

func (s *Server) RespondToRequest(req *Request) {
	if req.IsCancelled {
		return
	}

	trafficDelayFactor := float64(req.RouteTimeRT() / req.RouteTime())
	trafficDuration := req.TrafficDuration()

	if trafficDelayFactor <= maxTrafficDelayFactor {
		var trafficString string
		if trafficDuration.Minutes() < 1 {
			trafficString = "Traffic: absolutely none!"
		} else if trafficDelayFactor < (busyTrafficDelayFactor) {
			trafficString = fmt.Sprintf("Traffic: bearable, %.0fm delay", trafficDuration.Minutes())
		} else {
			trafficString = fmt.Sprintf("Traffic: getting busy, %.0fm delay", trafficDuration.Minutes())
		}

		msg := req.MessageText(fmt.Sprintf("GO! %.0fm drive. %s", req.RouteTimeRT().Minutes(), trafficString))
		s.reply(req, msg)
		return
	}

	if !req.IsRetrying {
		req.IsRetrying = true
		msg := req.MessageText(fmt.Sprintf("WAIT. %.0fm of traffic (%.0fm total). I'll notify you when clear.", trafficDuration.Minutes(), req.RouteTimeRT().Minutes()))
		s.reply(req, msg)
	}

	log.Printf("Route has traffic delay of %f mins. Will retry.\n", trafficDuration.Minutes())
	s.reprocessAfter(req, defaultReprocessDelay)
}

func (s *Server) reply(req *Request, msg string) {
	if req.Type == RequestTypeTweet {
		s.postTweet(req.User, msg)
	} else {
		s.sendDM(req.User, msg)
	}
}

func (s *Server) postTweet(to anaconda.User, text string) {
	_, err := s.api.PostTweet(ReplyTweetMessage(to, text), url.Values{})
	if err != nil {
		log.Println("Problem posting tweet", err)
	}
}

func (s *Server) sendDM(to anaconda.User, text string) {
	_, err := s.api.PostDMToUserId(text, to.Id)
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

func (s *Server) reprocessAfter(req *Request, delay time.Duration) {
	if req.IsCancelled {
		return
	}

	go func() {
		timer := time.NewTimer(delay)
		defer func() {
			if !timer.Stop() {
				<-timer.C
			}
		}()

		select {
		case <-req.CancelRetry:
			return
		case <-timer.C:
			s.requestQueue <- req
			return
		}
	}()
}
