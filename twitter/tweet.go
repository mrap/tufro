package twitter

import (
	"github.com/ChimeraCoder/anaconda"
	"github.com/mrap/waze/location"
)

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
