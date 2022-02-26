package nasa_epic_api

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"sort"
	"time"
)

func CreateDBRecordType(identifier, formattedDateString, imageLocation string, size int64, date time.Time) DBRecord {
	return DBRecord{
		Identifier:       identifier,
		FormattedDateStr: formattedDateString,
		Date:             date,
		ImageSize:        size,
		S3Location:       imageLocation,
	}
}

func CreateDBClient(region string) (*dynamodb.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	client := dynamodb.NewFromConfig(cfg)

	return client, nil
}

func WriteDBItem(client *dynamodb.Client, item DBRecord, tableName string) error {
	i, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("unable to marshal map when writing DB item: %v", err)
	}

	putItemInput := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      i,
	}

	_, err2 := client.PutItem(context.TODO(), putItemInput)
	if err2 != nil {
		return fmt.Errorf("error for PutItem against %s: %v", tableName, err2)
	}

	fmt.Printf("Successfully wrote item %s to DB\n", item.Identifier)

	return nil
}

func CheckIfDBItemExists(client *dynamodb.Client, identifier, date string, tableName string) (bool, error) {
	getItemInput := &dynamodb.GetItemInput{
		TableName: &tableName,

		Key: map[string]types.AttributeValue{
			"Identifier": &types.AttributeValueMemberS{
				Value: identifier,
			},
			"FormattedDateStr": &types.AttributeValueMemberS{
				Value: date,
			},
		},
	}

	result, err := client.GetItem(context.TODO(), getItemInput)
	if err != nil {
		return false, err
	}

	// check whether the item has been found
	if len(result.Item) > 0 {
		return true, nil
	} else {
		return false, nil
	}
}

// RetrieveAllItemsAsStruct reads all items from the database and returns a []NasaEpicRecording
func RetrieveAllItemsAsStruct(client *dynamodb.Client, tableName string) ([]*NasaEpicRecording, error) {
	var results []*NasaEpicRecording

	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	}

	scanOutput, err := client.Scan(context.TODO(), scanInput)
	if err != nil {
		return nil, fmt.Errorf("unable to scan table: %s", tableName)
	}

	if scanOutput.Count > 0 {
		var item *NasaEpicRecording

		for _, value := range scanOutput.Items {
			err = attributevalue.UnmarshalMap(value, &item)
			if err != nil {
				return nil, fmt.Errorf("unable to unmarshal map: %v", value)
			}

			results = append(results, item)
			item = nil
		}
	} else {
		return nil, nil
	}

	// sort records based on date/time
	sort.Slice(results, func(i, j int) bool {
		return results[i].Date.Before(results[j].Date)
	})

	return results, nil
}
