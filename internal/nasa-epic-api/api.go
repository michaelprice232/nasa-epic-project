package nasa_epic_api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	baseAPIURL = "https://epic.gsfc.nasa.gov"
)

func NewDateSlice() []*Date {
	return []*Date{}
}

func ProcessRecordingDates(dbclient *dynamodb.Client, tableName string, s3client *s3.Client, bucketName string,
	dates []*Date, startDate time.Time, targetCoordinatesRange map[string]float64) ([]*NasaEpicRecording, error) {

	var nasaRecordsAllMatchedCoordinates []*NasaEpicRecording

	datesToProcess := AvailableDatesToTarget(dates, startDate)

	for _, recordingDate := range datesToProcess {

		var nasaRecordsForSingleDay []*NasaEpicRecording = nil

		formattedDate := recordingDate.Date.Format("2006-01-02") // format used in API URI
		targetURL := baseAPIURL + "/api/natural/date/" + formattedDate
		fmt.Printf("\nretrieving url: %s\n", targetURL)

		data, err := GetHTTP(targetURL)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve url %s: %v", targetURL, err)
		}

		err = json.Unmarshal(data, &nasaRecordsForSingleDay)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal data: %v", err)
		}

		UpdateDateFieldRecordings(nasaRecordsForSingleDay, "2006-01-02 15:04:05")

		matchedCoordinateResults := QueryRecordingsOnGeoLocation(nasaRecordsForSingleDay, targetCoordinatesRange)

		newlyDiscoveredRecords, err2 := ProcessRecordings(dbclient, tableName, s3client, bucketName, matchedCoordinateResults, recordingDate)
		if err2 != nil {
			return nil, fmt.Errorf("problem within the ProcessRecordings function: %v", err2)
		}

		nasaRecordsAllMatchedCoordinates = append(nasaRecordsAllMatchedCoordinates, newlyDiscoveredRecords...)
	}

	return nasaRecordsAllMatchedCoordinates, nil
}

func ProcessRecordings(dbclient *dynamodb.Client, tableName string, s3client *s3.Client, bucketName string,
	recordings []*NasaEpicRecording, recordingDate *Date) ([]*NasaEpicRecording, error) {

	var newlyDiscoveredRecords []*NasaEpicRecording

	for _, recording := range recordings {

		dateFormat := "2006-01-02"
		dateTimeFormat := "2006-01-02 03:04PM"
		formattedDate := recording.Date.Format(dateFormat)
		formattedDateTime := recording.Date.Format(dateTimeFormat)

		// pad month/day to avoid URL issues with single digits
		paddedMonth := fmt.Sprintf("%02d", recordingDate.Date.Month())
		paddedDay := fmt.Sprintf("%02d", recordingDate.Date.Day())

		filename := recording.Image + ".png"
		downloadDestinationPath := "/tmp/" + filename
		targetS3KeyName := formattedDate + "/" + filename

		imageDownloadLocation := fmt.Sprintf("%s/archive/natural/%d/%s/%s/png/%s",
			baseAPIURL,
			recordingDate.Date.Year(),
			paddedMonth,
			paddedDay,
			filename)

		// check whether the item exists in the DB first already and do not download image
		found, err := CheckIfDBItemExists(dbclient, recording.Identifier, formattedDateTime, tableName)
		if err != nil {
			return nil, fmt.Errorf("unable to check if item already exists in DB: %v", err)
		}

		if !found {
			// download the image locally from the nasa server first
			size, err2 := DownloadImage(imageDownloadLocation, downloadDestinationPath)
			if err2 != nil {
				return nil, fmt.Errorf("unable to download image %s: %v", downloadDestinationPath, err2)
			}

			// upload to S3
			file, err3 := os.Open(downloadDestinationPath)
			if err3 != nil {
				return nil, fmt.Errorf("unable to open file %s: %v", downloadDestinationPath, err3)
			}

			s3Location, err4 := UploadS3Object(s3client, file, bucketName, targetS3KeyName, "image/png")
			if err4 != nil {
				return nil, fmt.Errorf("unable to upload file to S3: %v", err4)
			}

			// close and then clean up local copy of file
			err5 := file.Close()
			if err5 != nil {
				return nil, fmt.Errorf("unable to close file %s: %v", downloadDestinationPath, err5)
			}
			err = os.Remove(downloadDestinationPath)
			if err != nil {
				return nil, fmt.Errorf("unable to remove local copy of file %s", downloadDestinationPath)
			}

			// update struct with additional information required for later HTML templating to S3 bucket
			recording.S3Location = s3Location
			recording.FormattedDateStr = formattedDateTime
			recording.ImageSize = size

			// write to database after completing successfully
			record := CreateDBRecordType(recording.Identifier, formattedDateTime, s3Location, size, recording.Date)
			err = WriteDBItem(dbclient, record, tableName)
			if err != nil {
				return nil, fmt.Errorf("error writing record '%v' to database: %v", record, err)
			}

			// append to return results
			newlyDiscoveredRecords = append(newlyDiscoveredRecords, recording)

		} else {
			fmt.Printf("Skipping as item %s already present in database\n", recording.Identifier)
		}
	}
	return newlyDiscoveredRecords, nil
}

func ConvertRawStringToDateTime(raw, format string) time.Time {
	// we use the reference values from the time package to define our own format
	formattedDateTime, err := time.Parse(format, raw)
	if err != nil {
		fmt.Printf("unable to convert string %s into time format: %v", raw, err)
		panic(err)
	}
	return formattedDateTime
}

// UpdateDateFieldRecordings updates all the Date fields based on the DateString date field
func UpdateDateFieldRecordings(slice []*NasaEpicRecording, format string) {
	for i := 0; i < len(slice); i++ {
		slice[i].Date = ConvertRawStringToDateTime(slice[i].DateString, format)
	}
}

// UpdateDateFieldDates updates all the Date fields based on the DateString date field
// todo: duplicate function to above minus the types. can we abstract with an interface?
func UpdateDateFieldDates(slice []*Date) {
	dateFormat := "2006-01-02"
	for i := 0; i < len(slice); i++ {
		// set the formatted time.Date struct field
		slice[i].Date = ConvertRawStringToDateTime(slice[i].DateString, dateFormat)
	}
}

func QueryRecordingsOnGeoLocation(slice []*NasaEpicRecording, coordinates map[string]float64) []*NasaEpicRecording {
	var resultsSlice []*NasaEpicRecording
	//fmt.Printf("length of original slice: %d\n", len(slice))
	for i := 0; i < len(slice); i++ {
		// check if the coordinates are within the min/max thresholds for latitude and longitude
		if slice[i].CentroidCoordinates.Lat >= coordinates["latMin"] &&
			slice[i].CentroidCoordinates.Lat <= coordinates["latMax"] &&
			slice[i].CentroidCoordinates.Lon >= coordinates["lonMin"] &&
			slice[i].CentroidCoordinates.Lon <= coordinates["lonMax"] {
			resultsSlice = append(resultsSlice, slice[i])
		}
	}
	fmt.Printf("number of coordinates matched: %d\n", len(resultsSlice))

	return resultsSlice
}

func GetStartDate(dayRangeStr string) time.Time {
	dayRange, err := strconv.Atoi(dayRangeStr)
	if err != nil {
		panic("unable to calculate the GetStartDate start date")
	}

	// subtract DAY_RANGE from the current date
	dateTimeRangeStart := time.Now().Add(-(time.Hour * 24 * time.Duration(dayRange)))

	// strip the time element, so we can search from the start of the day
	dateTimeRangeStartTrunc := dateTimeRangeStart.Truncate(time.Hour * 24)
	fmt.Printf("selecting recordings since date: %v\n", dateTimeRangeStartTrunc)

	return dateTimeRangeStartTrunc
}

func AvailableDatesToTarget(slice []*Date, startDate time.Time) []*Date {
	var targetDates []*Date
	for _, date := range slice {
		if date.Date.After(startDate) || date.Date.Equal(startDate) {
			//fmt.Printf("adding date %v\n", date)
			targetDates = append(targetDates, date)
		}
	}
	return targetDates
}

func GetHTTP(url string) ([]byte, error) {
	// workaround for web proxy interception
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("non-200 status code: %d", resp.StatusCode)
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			fmt.Printf("unable to close file in closure: %v", err)
			panic(err)
		}
	}(resp.Body)
	body, err2 := io.ReadAll(resp.Body)
	if err2 != nil {
		return nil, fmt.Errorf("unable to read HTTP response data: %v", err2)
	}

	//fmt.Printf("Server header: %s\n", resp.Header.Get("Server"))

	return body, nil
}

// DownloadImage todo: how do we update this closure to return error instead of panic?
func DownloadImage(url, destination string) (int64, error) {
	response, err := GetHTTP(url)
	if err != nil {
		return 0, err
	}

	// create an empty file
	file, err2 := os.Create(destination)
	if err2 != nil {
		return 0, err2
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			fmt.Printf("unable to close in the download image closure: %v", err)
			panic(err)
		}
	}(file)

	size, err3 := io.Copy(file, bytes.NewReader(response))
	if err3 != nil {
		return 0, err3
	}
	fmt.Printf("%s has been successfully downloaded to: %s\n", url, destination)

	return size, nil
}

//func PrintStats(slice []*NasaEpicRecording, coordinateMatches int) {
//	var latitudeRanges []float64
//	var longitudeRanges []float64
//
//	fmt.Println("\nStats:")
//	for _, recording := range slice {
//		latitudeRanges = append(latitudeRanges, recording.CentroidCoordinates.Lat)
//		longitudeRanges = append(longitudeRanges, recording.CentroidCoordinates.Lon)
//
//		// Print all records:
//		//fmt.Printf("date: %v\t coordinates: %v\n", recording.DateString, recording.CentroidCoordinates)
//	}
//
//	if latitudeRanges != nil && longitudeRanges != nil {
//		sort.Sort(sort.Float64Slice(longitudeRanges))
//		sort.Sort(sort.Float64Slice(latitudeRanges))
//
//		fmt.Printf("min/max latitude: %f/%f\n", latitudeRanges[0], latitudeRanges[len(latitudeRanges)-1])
//		fmt.Printf("min/max longitude: %f/%f\n", longitudeRanges[0], longitudeRanges[len(longitudeRanges)-1])
//		fmt.Printf("coordinate matches: %d\n", coordinateMatches)
//	}
//}

func GetAllAvailableDates() ([]byte, error) {
	data, err := GetHTTP(baseAPIURL + "/api/natural/all")
	if err != nil {
		return nil, err
	}
	return data, nil
}

// todo: enable once debug logging enabled
//func printCoordinates(slice []NasaEpicRecording, count uint32) {
//	if len(slice) > 0 {
//		if count == 0 {
//			fmt.Printf("\nPrinting the coordinates of all recordings in the passed slice:\n")
//			for i := 0; i < len(slice); i++ {
//				fmt.Println(slice[i].CentroidCoordinates)
//			}
//		} else {
//			// protect against out of index bounds
//			if int(count) > len(slice) {
//				count = uint32(len(slice))
//			}
//			fmt.Printf("\nPrinting the coordinates of the first %d recordings in the passed slice:\n", count)
//			for i := 0; i < int(count); i++ {
//				fmt.Println(slice[i].CentroidCoordinates)
//			}
//		}
//	}
//}

// todo: enable once debug logging enabled
//func printDates(slice []Date, count uint32) {
//	if count == 0 {
//		fmt.Printf("\nPrinting the dates of all recordings in the passed slice:\n")
//		for i := 0; i < len(slice); i++ {
//			fmt.Println(slice[i].Date)
//		}
//	} else {
//		// protect against out of index bounds
//		if int(count) > len(slice) {
//			count = uint32(len(slice))
//		}
//		fmt.Printf("\nPrinting the first %d dates of available recordings in the passed slice:\n", count)
//		for i := 0; i < int(count); i++ {
//			fmt.Println(slice[i].Date)
//		}
//	}
//	fmt.Println()
//}
