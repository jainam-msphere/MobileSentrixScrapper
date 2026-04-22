package database

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const TableName = "ScrappedDevicesData"

type Client struct {
	Db *dynamodb.Client
}

func NewClient() *Client {
	db_cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-south-1"), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummysecret", "")))
	if err != nil {
		log.Fatalf("failed to create AWS session: %w", err)
		return nil
	}
	dynamoClient := dynamodb.NewFromConfig(db_cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String("http://localhost:8000")
	})
	return &Client{Db: dynamoClient}
}

func (client *Client) CreateTable() error {
	_, err := client.Db.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		TableName: aws.String(TableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("deviceName"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("deviceBrand"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("deviceName"),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String("deviceBrand"),
				KeyType:       types.KeyTypeRange,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		fmt.Printf("Table note: %v\n", err)
		return err
	} else {
		fmt.Println("Scrapper data table created successfully")
	}
	return nil
}

func (client *Client) CreateDeviceTable() error {
	_, err := client.Db.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		TableName: aws.String("DevicesList"),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("brandName"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("brandName"),
				KeyType:       types.KeyTypeHash,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		fmt.Printf("Table note: %v\n", err)
		return err
	} else {
		fmt.Println("Device table created successfully")
	}
	return nil
}
func (client *Client) CreateBrandTable() error {
	_, err := client.Db.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
		TableName: aws.String("BrandList"),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("keyName"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("keyName"),
				KeyType:       types.KeyTypeHash,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		fmt.Printf("Table note: %v\n", err)
		return err
	} else {
		fmt.Println("Brands table created successfully")
	}

	return nil
}
