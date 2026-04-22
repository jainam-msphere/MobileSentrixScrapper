package handler

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/valyala/fasthttp"
	"scrapper.com/internals/utils"
	"scrapper.com/models"
)

func (h *HandlerDb) GetDevices(ctx *fasthttp.RequestCtx) {

	manufacturerVal := ctx.UserValue("manufacturer_name")
	manufacturerEncoded, ok := manufacturerVal.(string)
	if !ok || manufacturerEncoded == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	manufacturer, err := url.PathUnescape(manufacturerEncoded)
	if err != nil || manufacturer == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}

	pageToken := string(ctx.QueryArgs().Peek("page_token"))
	type data struct {
		Path         string   `json:"path"`
		Manufacturer string   `json:"manufacturer"`
		Devices      []string `json:"devices"`
	}
	var res struct {
		NextToken string `json:"next_page_token"`
		Results   []data `json:"results"`
	}

	var result *dynamodb.QueryOutput

	result, err = h.Db.Query(context.TODO(), &dynamodb.QueryInput{
		TableName:              aws.String("DevicesList"),
		KeyConditionExpression: aws.String("brandName = :brand"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":brand": &types.AttributeValueMemberS{Value: manufacturer},
		},
	})
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "database error", "database fetch failed")
		return
	}

	if result != nil && len(result.Items) > 0 {
		var phone []models.DevicesInfo
		err = attributevalue.UnmarshalListOfMaps(result.Items, &phone)
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
			return
		}
		res.NextToken = pageToken
		var temp data
		temp.Manufacturer = manufacturer
		temp.Path = "/manufacturers/" + manufacturer + "/devices"
		temp.Devices = phone[0].BrandDevicesList
		res.Results = append(res.Results, temp)
		response, err := json.Marshal(res)
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
			return
		}
		ctx.SetContentType("application/json")
		ctx.SetBody(response)
		return
	}

}
