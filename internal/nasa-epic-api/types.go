package nasa_epic_api

import (
	"time"
)

type NasaEpicRecording struct {
	Identifier          string
	Caption             string
	Image               string
	Version             string
	CentroidCoordinates Coordinates `json:"centroid_coordinates"`
	DateString          string      `json:"date"`
	Date                time.Time
	FormattedDateStr    string
	S3Location          string
	ImageSize           int64
}

type Coordinates struct {
	Lat float64
	Lon float64
}

type Date struct {
	DateString string `json:"date"`
	Date       time.Time
}

type DBRecord struct {
	Identifier       string
	FormattedDateStr string
	ImageSize        int64
	S3Location       string
	Date             time.Time
}

type recordingDetail struct {
	Recordings        []*NasaEpicRecording
	FavIconS3Location string
	WebsiteURL        string
}
