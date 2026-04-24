package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/valyala/fasthttp"
	"scrapper.com/internals"
	"scrapper.com/internals/utils"
	"scrapper.com/models"
)

func (h *HandlerDb) GetBrands(ctx *fasthttp.RequestCtx) {

	var result *dynamodb.QueryOutput
	result, err := h.Db.Query(context.TODO(), &dynamodb.QueryInput{
		TableName:              aws.String("BrandList"),
		KeyConditionExpression: aws.String("keyName = :val"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":val": &types.AttributeValueMemberS{Value: "brands"},
		},
	})

	if err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "database error", "database fetch failed"+err.Error())
		return
	}

	pageToken := string(ctx.QueryArgs().Peek("page_token"))
	type data struct {
		Path         string `json:"path"`
		Manufacturer string `json:"manufacturer"`
	}
	var res struct {
		NextToken string `json:"next_page_token"`
		Results   []data `json:"results"`
	}

	if result != nil && len(result.Items) > 0 {
		var phone []models.BrandsInfo
		err = attributevalue.UnmarshalListOfMaps(result.Items, &phone)
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
			return
		}
		res.NextToken = pageToken
		for _, v := range phone[0].BrandList {
			var temp data
			temp.Manufacturer = v
			temp.Path = "/manufacturers/" + v + "/devices"
			res.Results = append(res.Results, temp)
		}
		response, err := json.Marshal(res)
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
			return
		}
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBody(response)
		return
	} else {
		brandList, err := internals.FindBrandsInGSM()
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
			return
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
			res.NextToken = pageToken
			for _, v := range brands.BrandList {
				var temp data
				temp.Manufacturer = v
				temp.Path = "/manufacturers/" + v + "/devices"
				res.Results = append(res.Results, temp)
			}
			response, err := json.Marshal(res)
			if err != nil {
				utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
				return
			}
			ctx.SetContentType("application/json")
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBody(response)
			fmt.Println("migrated brands")
		}
	}
}
