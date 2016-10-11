package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mrap/waze/location"
	"github.com/mrap/waze/route"
)

func main() {
	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		from := c.Query("from")
		to := c.Query("to")
		if to == "" || from == "" {
			c.String(http.StatusBadRequest, "")
			return
		}

		fromLoc := location.SearchTopLocation(from, nil)
		if fromLoc == nil {
			c.String(http.StatusNotFound, "Unable to find [from] location: '%s'", from)
			return
		}
		toLoc := location.SearchTopLocation(to, fromLoc.Coordinates)
		if toLoc == nil {
			c.String(http.StatusNotFound, "Unable to find [to] location: '%s'", to)
			return
		}

		routes, err := route.GetRoutes(fromLoc, toLoc)
		if err != nil || len(routes) == 0 {
			c.String(http.StatusInternalServerError, "Unable to find routes from '%s' to '%s'", from, to)
			return
		}

		c.String(http.StatusOK, "%d", routes[0].TotalTime())
	})

	router.Run(":3000")
}
