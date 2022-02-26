package nasa_epic_api

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// CreateSESClient returns an *sesv2.Client
func CreateSESClient(region string) (*sesv2.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	client := sesv2.NewFromConfig(cfg)

	return client, nil
}

// SendEmail sends an email via the AWS SES v2 API
func SendEmail(client *sesv2.Client, sender string, recipients []string, subject, body, htmlBody string) (string, error) {
	destinationInput := &types.Destination{}
	for _, recipient := range recipients {
		destinationInput.BccAddresses = append(destinationInput.BccAddresses, recipient)
	}

	fmt.Printf("\nSending to recipients: %+v\n", destinationInput.BccAddresses)

	contentInput := &types.EmailContent{
		Simple: &types.Message{
			Subject: &types.Content{
				Data: aws.String(subject),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data: aws.String(htmlBody),
				},
				Text: &types.Content{
					Data: aws.String(body),
				},
			},
		},
	}

	sendEmailInput := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(sender),
		Destination:      destinationInput,
		Content:          contentInput,
	}

	output, err := client.SendEmail(context.TODO(), sendEmailInput)
	if err != nil {
		return "", fmt.Errorf("unable to send ses message: %v", err)
	}

	return aws.ToString(output.MessageId), nil
}

// SendEmailReport generates an HTML report based on []*NasaEpicRecording and then calls SendEmail
func SendEmailReport(recordings []*NasaEpicRecording, client *sesv2.Client, sender string, recipients []string, websiteURL string) error {
	sourceTemplateFile := "internal/nasa-epic-api/templates/emailReport.tmpl"

	sourceFile, err := ioutil.ReadFile(sourceTemplateFile)
	if err != nil {
		return fmt.Errorf("unable to open source template file %s: %v", sourceTemplateFile, err)
	}

	index := template.Must(template.New("emailReport").Parse(string(sourceFile)))

	recordingDetails := recordingDetail{
		recordings,
		"",
		websiteURL,
	}

	// parse HTML to buffer instead of file so that we can extract as string
	var report bytes.Buffer

	err = index.Execute(&report, recordingDetails)
	if err != nil {
		return fmt.Errorf("unable to execute the templating action: %v", err)
	}

	bodyText := fmt.Sprintf("%+v", recordings)
	bodyHTML := report.String()

	messageID, err := SendEmail(client, sender, recipients, "Nasa Epic Coordinate Matches", bodyText, bodyHTML)
	if err != nil {
		return fmt.Errorf("problems sending email: %v", err)
	}
	fmt.Printf("\nEmail sent OK. Message ID: %s\n", messageID)

	return nil
}
