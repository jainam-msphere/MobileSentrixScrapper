package handler

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"scrapper.com/database"
	"scrapper.com/internals"
	"scrapper.com/internals/utils"
	"scrapper.com/models"
)

type HandlerDb struct {
	Db *dynamodb.Client
}

func (h *HandlerDb) CreatePhoneItem(ctx *fasthttp.RequestCtx) {
	var phoneData models.ScrapData
	if err := json.Unmarshal(ctx.PostBody(), &phoneData); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString(`{"error": "invalid request body"}`)
		return
	}

	phoneData.DeviceId = uuid.New().String()

	item, err := attributevalue.MarshalMap(phoneData)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.SetBodyString(`{"error": "failed to marshal book"}`)
		return
	}

	_, err = h.Db.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(database.TableName),
		Item:      item,
	})
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.WriteString(`{"error": "` + err.Error() + `"}`)
		return
	}

	ctx.SetStatusCode(fasthttp.StatusCreated)
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(phoneData)
}

func (h *HandlerDb) GetPhoneItem(ctx *fasthttp.RequestCtx) {

	itemVal := ctx.UserValue("item_name")
	itemEncoded, ok := itemVal.(string)
	if !ok || itemEncoded == "" {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "device name not found")
		return
	}
	item, err := url.PathUnescape(itemEncoded)
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "device name invalid")
		return
	}

	brandVal := ctx.UserValue("brand_name")
	brandEncoded, ok := brandVal.(string)
	if !ok || brandEncoded == "" {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "brand name not found")
		return
	}
	brand, err := url.PathUnescape(brandEncoded)
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "brand name invalid")
		return
	}

	sourceVal := ctx.UserValue("source_type")
	sourceEncoded, ok := sourceVal.(string)
	if !ok || sourceEncoded == "" {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "source type not found")
		return
	}
	variant, err := url.PathUnescape(sourceEncoded)
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "source type invalid")
		return
	}

	source := string(ctx.QueryArgs().Peek("source_name"))
	link := string(ctx.QueryArgs().Peek("link"))

	if variant != "" && variant != "link" && variant != "data" {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "variant type invalid")
		return
	}
	if variant == "link" && link == "" {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "variant or link params invalid")
		return
	}
	if brand == "" {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "brand name invalid")
		return
	}
	if variant == "link" && (source != "gsmarena" && source != "phonedb") {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "source type invalid")
		return
	}

	var result *dynamodb.QueryOutput

	result, err = h.Db.Query(context.TODO(), &dynamodb.QueryInput{
		TableName:              aws.String(database.TableName),
		KeyConditionExpression: aws.String("deviceName = :device AND deviceBrand = :brand"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":device": &types.AttributeValueMemberS{Value: item},
			":brand":  &types.AttributeValueMemberS{Value: brand},
		},
	})
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "database error", "database fetch failed")
		return
	}

	if result != nil && len(result.Items) > 0 {

		var phone []models.ScrapData
		err = attributevalue.UnmarshalListOfMaps(result.Items, &phone)
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "database error", "database read failed")
			return
		}

		response, err := json.Marshal(phone[0])
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "database error", "database read failed")
			return
		}
		ctx.SetContentType("application/json")
		ctx.SetBody(response)
		return
	}
	var scrapedData []byte
	if variant == "link" && link != "" {
		link, err = url.PathUnescape(link)
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusBadRequest, "params error", "invalid link provided")
			return
		}
		if source == "gsmarena" {
			scrapedData, err = internals.PrintSpecList(link, item, brand)
			if err != nil {
				utils.SendError(ctx, fasthttp.StatusInternalServerError, "process error", "failed scrapping data")
				return
			}
		}
		if source == "phonedb" {
			html, err := internals.FetchDetailTable(internals.PhoneResult{Title: item, DetailHref: link})
			if err != nil {
				utils.SendError(ctx, fasthttp.StatusInternalServerError, "process error", "failed scrapping data")
				return
			}
			temp := internals.BodyObj{HtmlString: html, PhoneName: item, CompanyName: brand}
			scrapedData, err = internals.PDBParser(temp)
			if err != nil {
				utils.SendError(ctx, fasthttp.StatusInternalServerError, "process error", "failed scrapping data")
				return
			}
		}
	} else {
		scrapedData, err = internals.FetchDataGSM(item, brand)
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "process error", "failed scrapping data")
			return
		}
	}

	var parsed map[string]any
	if err := json.Unmarshal(scrapedData, &parsed); err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
		return
	}

	phoneData := models.ScrapData{
		DeviceId:    uuid.New().String(),
		DeviceName:  item,
		DeviceBrand: brand,
		DeviceInfo:  parsed,
	}
	phoneData.Path = string(ctx.Path())
	itemMap, err := attributevalue.MarshalMap(phoneData)
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
		return
	}

	_, err = h.Db.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(database.TableName),
		Item:      itemMap,
	})
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
		return
	}
	response, err := json.Marshal(phoneData)
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetBody(response)
}

// func (h *HandlerDb) UpdatePhoneItem(ctx *fasthttp.RequestCtx) {
// 	itemVal := ctx.UserValue("item_name")
// 	name, ok := itemVal.(string)
// 	if !ok || name == "" {
// 		ctx.SetStatusCode(fasthttp.StatusBadRequest)
// 		return
// 	}
// 	var newPhoneScrapData models.ScrapData
// 	if err := json.Unmarshal(ctx.PostBody(), &newPhoneScrapData); err != nil {
// 		ctx.SetStatusCode(fasthttp.StatusBadRequest)
// 		ctx.SetBodyString(`{"error": "invalid request body"}`)
// 		return
// 	}
// 	dVal, err := attributevalue.Marshal(newPhoneScrapData.DeviceInfo)
// 	if err != nil {
// 		ctx.SetStatusCode(fasthttp.StatusBadRequest)
// 		return
// 	}
// 	_, err = h.Db.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
// 		TableName: aws.String(database.TableName),
// 		Key: map[string]types.AttributeValue{
// 			"id": &types.AttributeValueMemberS{Value: name},
// 		},
// 		UpdateExpression: aws.String("SET DeviceInfo = :d"),
// 		ExpressionAttributeValues: map[string]types.AttributeValue{
// 			":d": dVal,
// 		},
// 		ReturnValues: types.ReturnValueAllNew,
// 	})
// 	if err != nil {
// 		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
// 		ctx.WriteString(`{"error": "` + err.Error() + `"}`)
// 		return
// 	}
// 	ctx.SetContentType("application/json")
// 	json.NewEncoder(ctx).Encode(newPhoneScrapData)
// }

func (h *HandlerDb) HealthChecker(ctx *fasthttp.RequestCtx) {
	if h.Db != nil {
		ctx.SetStatusCode(200)
		ctx.WriteString("server ok")
		return
	} else {
		ctx.SetStatusCode(500)
		ctx.WriteString("some error occured in server")
		return
	}
}
