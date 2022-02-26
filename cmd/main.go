package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"nasa-epic-project/internal/nasa-epic-api"

	"github.com/aws/aws-lambda-go/lambda"
)

var (
	dbTableName            string
	uploadS3BucketName     string
	region                 string
	emailSender            string
	emailRecipientsStr     string
	dayRangeStr            string
	targetCoordinatesRange = map[string]float64{}

	emailRecipients []string
)

func init() {
	// load all envars
	dbTableName = loadEnvar("dbTableName")
	uploadS3BucketName = loadEnvar("uploadS3BucketName")
	region = loadEnvar("region")
	emailSender = loadEnvar("emailSender")
	emailRecipientsStr = loadEnvar("emailRecipientsStr")
	dayRangeStr = loadEnvar("dayRangeStr")

	var err error
	targetCoordinatesRange["latMin"], err = strconv.ParseFloat(loadEnvar("targetCoordinateslatMin"), 64)
	if err != nil {
		log.Fatalf("unable to parse float64 for latMin: %v", err)
	}

	targetCoordinatesRange["latMax"], err = strconv.ParseFloat(loadEnvar("targetCoordinateslatMax"), 64)
	if err != nil {
		log.Fatalf("unable to parse float64 for latMax: %v", err)
	}

	targetCoordinatesRange["lonMin"], err = strconv.ParseFloat(loadEnvar("targetCoordinateslonMin"), 64)
	if err != nil {
		log.Fatalf("unable to parse float64 for lonMin: %v", err)
	}

	targetCoordinatesRange["lonMax"], err = strconv.ParseFloat(loadEnvar("targetCoordinateslonMax"), 64)
	if err != nil {
		log.Fatalf("unable to parse float64 for lonMax: %v", err)
	}
}

// loadEnvar looks up an environment variable and exits the program if not found
func loadEnvar(envarName string) string {
	var exists bool
	var value string
	value, exists = os.LookupEnv(envarName)
	if !exists {
		log.Fatalf("unable to load envar: %s.Exiting", envarName)
	}
	return value
}

func handler() {
	websiteURL := fmt.Sprintf("http://%s.s3-website-%s.amazonaws.com", uploadS3BucketName, region)

	dbclient, err := nasa_epic_api.CreateDBClient(region)
	if err != nil {
		panic(err)
	}

	s3Client, err4 := nasa_epic_api.CreateS3Client()
	if err4 != nil {
		panic(err4)
	}

	err = nil
	sesclient, err := nasa_epic_api.CreateSESClient(region)
	if err != nil {
		panic(err)
	}

	// populate slice of email recipients based on envar source
	emailRecipients = strings.Split(emailRecipientsStr, ",")

	availableRecordingDates := nasa_epic_api.NewDateSlice()

	startDate := nasa_epic_api.GetStartDate(dayRangeStr)

	data, err2 := nasa_epic_api.GetAllAvailableDates()
	if err2 != nil {
		panic(err2)
	}

	err = json.Unmarshal(data, &availableRecordingDates)
	if err != nil {
		panic(err)
	}

	nasa_epic_api.UpdateDateFieldDates(availableRecordingDates)

	matchedCoordinateRecords, err3 := nasa_epic_api.ProcessRecordingDates(
		dbclient, dbTableName, s3Client, uploadS3BucketName,
		availableRecordingDates, startDate, targetCoordinatesRange)
	if err3 != nil {
		panic(err3)
	}

	// retrieve all records from database to generate HTML index file
	allDBRecords, err4 := nasa_epic_api.RetrieveAllItemsAsStruct(dbclient, dbTableName)
	if err4 != nil {
		log.Printf("problems building struct slice from database items: %v\n", err4)
	}

	fmt.Printf("\nFound %d items in the database. Building HTML Index...\n", len(allDBRecords))
	err = nasa_epic_api.GenerateHTMLIndex(allDBRecords, s3Client, uploadS3BucketName)
	if err != nil {
		log.Fatalf("an error occurred when attempting to generate the HTML content: %v\n", err)
	}

	// print coordinate matches from this run to the console
	if len(matchedCoordinateRecords) > 0 {
		fmt.Printf("\nPrinting coordinate matches from this run which were not already present in the database (%s days history):\n", dayRangeStr)
		for _, v := range matchedCoordinateRecords {
			fmt.Printf("Identifier: %+v, S3Location: %+v DateString: %+v\n", v.Identifier, v.S3Location, v.DateString)
		}

		// send email notifications as matches where found
		err = nil
		err := nasa_epic_api.SendEmailReport(matchedCoordinateRecords, sesclient, emailSender, emailRecipients, websiteURL)
		if err != nil {
			log.Fatalf("problems sending email: %v", err)
		}

	} else {
		fmt.Printf("\nNo coordinate matches in this run (%s days history)\n", dayRangeStr)
	}

	fmt.Printf("\nPublic static website available at: %s\n", websiteURL)

	// todo: add a logger
	// todo: stats commented out for now. Cleanup
	//nasa_epic_api.PrintStats(allRecordings, coordinateMatchesCount)
}

func main() {
	lambda.Start(handler)
}
