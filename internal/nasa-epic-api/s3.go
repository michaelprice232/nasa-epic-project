package nasa_epic_api

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func CreateS3Client() (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg)

	return client, nil
}

func UploadS3Object(client *s3.Client, sourceFile io.Reader, targetBucket, targetKey, contentType string) (string, error) {
	uploadOptions := &s3.PutObjectInput{
		Bucket:      aws.String(targetBucket),
		Key:         aws.String(targetKey),
		ContentType: aws.String(contentType),
		Body:        sourceFile,
	}

	uploader := manager.NewUploader(client)
	result, err := uploader.Upload(context.TODO(), uploadOptions)

	if err != nil {
		return "", err
	}

	fmt.Printf("uploaded object %s to S3 bucket %s\n", targetKey, targetBucket)

	return result.Location, nil
}

func GenerateHTMLIndex(recordings []*NasaEpicRecording, s3client *s3.Client, bucketName string) error {
	sourceIndexFile := "/tmp/index.html"
	DestinationIndexFile := "index.html"
	favIcon := "favicon.png"
	favIconPath := "images/" + favIcon
	sourceTemplateFile := "internal/nasa-epic-api/templates/index.tmpl"

	sourceFile, err := ioutil.ReadFile(sourceTemplateFile)
	if err != nil {
		return fmt.Errorf("unable to open source template file %s: %v", sourceTemplateFile, err)
	}

	index := template.Must(template.New("index").Parse(string(sourceFile)))

	// upload favicon to S3
	file, err2 := os.Open(favIconPath)
	if err2 != nil {
		return fmt.Errorf("unable to open favIcon file %s: %v", favIcon, err2)
	}
	defer file.Close()

	s3FavLocation, err3 := UploadS3Object(s3client, file, bucketName, favIcon, "image/png")
	if err3 != nil {
		return fmt.Errorf("unable to upload favicon to S3: %v", err3)
	}

	recordingDetails := recordingDetail{
		recordings,
		s3FavLocation,
		"",
	}

	file2, err4 := os.Create(sourceIndexFile)
	if err4 != nil {
		return fmt.Errorf("unable to create HTML index file: %v", err4)
	}
	defer file2.Close()

	err = index.Execute(file2, recordingDetails)
	if err != nil {
		return fmt.Errorf("unable to execute the templating action: %v", err)
	}

	file3, err5 := os.Open(sourceIndexFile)
	if err5 != nil {
		return fmt.Errorf("unable to open file %s: %v", sourceIndexFile, err5)
	}
	defer file3.Close()

	// upload to S3 to serve as static hosted website index file
	_, err6 := UploadS3Object(s3client, file3, bucketName, DestinationIndexFile, "text/html")
	if err6 != nil {
		return fmt.Errorf("unable to upload index file to S3: %v", err6)
	}

	return nil
}
