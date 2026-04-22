package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"scrapper.com/internals/handler"
	"scrapper.com/models"
)

func InitializeDbData(h *handler.HandlerDb) {
	var phones = make(map[string][]string)
	var brandList []string
	wd, _ := os.Getwd()
	fmt.Println("Working dir:", wd)
	data, err := os.ReadFile("migration/phoneList.json")
	if err != nil {
		log.Fatal(err)
		return
	}
	err = json.Unmarshal(data, &phones)
	if err != nil {
		log.Fatal(err)
		return
	}
	for k, v := range phones {
		var data models.DevicesInfo
		data.BrandName = k
		data.BrandDevicesList = v
		brandList = append(brandList, k)
		item, err := attributevalue.MarshalMap(data)
		if err != nil {
			log.Fatal(err)
			return
		}
		_, err = h.Db.PutItem(context.TODO(), &dynamodb.PutItemInput{
			TableName:           aws.String("DevicesList"),
			Item:                item,
			ConditionExpression: aws.String("attribute_not_exists(brandName)"),
		})
		if err != nil {
			var alreadyExist *types.ConditionalCheckFailedException
			if errors.As(err, &alreadyExist) {
				fmt.Println(k, "'s devices already exist")
			} else {
				log.Fatal(err)
				return
			}
		} else {
			fmt.Println("migrated", k, "'s devices")
		}
	}
	brands := models.BrandsInfo{
		KeyName:   "brands",
		BrandList: brandList,
	}
	item, err := attributevalue.MarshalMap(brands)
	if err != nil {
		log.Fatal(err)
		return
	}
	_, err = h.Db.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName:           aws.String("BrandList"),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(keyName)"),
	})
	if err != nil {
		var alreadyExist *types.ConditionalCheckFailedException
		if errors.As(err, &alreadyExist) {
			fmt.Println("brands already exist")
		} else {
			log.Fatal(err)
			return
		}
	} else {
		fmt.Println("migrated brands")
	}

}
