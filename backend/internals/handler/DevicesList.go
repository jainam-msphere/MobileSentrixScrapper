package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/html"
	"scrapper.com/internals"
	"scrapper.com/internals/utils"
	"scrapper.com/models"
)

const baseURL = "https://www.gsmarena.com/"

func (h *HandlerDb) GetDevices(ctx *fasthttp.RequestCtx) {

	type data struct {
		Path         string   `json:"path"`
		Manufacturer string   `json:"manufacturer"`
		Devices      []string `json:"devices"`
	}
	var res struct {
		NextToken string `json:"next_page_token"`
		Results   []data `json:"results"`
	}
	pageToken := string(ctx.QueryArgs().Peek("page_token"))
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
		temp.Path = "/manufacturers/" + phone[0].BrandName + "/devices"
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

	brandURL, err := internals.FindBrandUrl(manufacturer)
	if err != nil {
		log.Println(err)
		return
	}

	t := unique(findDevices(brandURL))

	var dbdata models.DevicesInfo
	dbdata.BrandName = manufacturer
	dbdata.BrandDevicesList = t
	item, err := attributevalue.MarshalMap(dbdata)
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
			log.Println(manufacturer, "'s devices already exist")
		} else {
			log.Fatal(err)
			return
		}
	} else {
		log.Println("saved", manufacturer, "'s devices in db")
	}

	res.NextToken = pageToken
	var temp data
	temp.Manufacturer = manufacturer
	temp.Path = "/manufacturers/" + manufacturer + "/devices"
	temp.Devices = t
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

func findDevices(brandURL string) []string {
	currentURL := brandURL
	page := 1
	var finalRes []string
	for {
		doc, err := internals.FetchHTML(currentURL)
		if err != nil {
			return []string{}
		}
		result, nextLink := scanPage(doc)
		finalRes = append(finalRes, result...)
		result = []string{}
		if nextLink != "" {
			currentURL = nextLink
			page++
			time.Sleep(1000 * time.Millisecond)
			continue
		}

		break
	}
	return finalRes
	// return "", fmt.Errorf("device %q not found under brand", deviceName)
}

func scanPage(doc *html.Node) ([]string, string) {
	var pageResult struct {
		Devices []string
	}
	makersDiv := internals.FindFirst(doc, func(n *html.Node) bool {
		return internals.IsElement(n, "div") && internals.HasClass(n, "makers")
	})
	if makersDiv == nil {
		return []string{}, ""
	}

	items := internals.FindAll(makersDiv, func(n *html.Node) bool {
		return internals.IsElement(n, "li")
	})

	for _, li := range items {
		a := internals.FindFirst(li, func(n *html.Node) bool { return internals.IsElement(n, "a") })
		if a == nil {
			continue
		}
		span := internals.FindFirst(a, func(n *html.Node) bool { return internals.IsElement(n, "span") })
		if span == nil {
			continue
		}
		name := strings.TrimSpace(internals.TextContent(span))
		nameClean := strings.ToLower(strings.TrimSpace(name))

		if nameClean != "" {
			pageResult.Devices = append(pageResult.Devices, nameClean)
		}
	}

	paginationLinks := internals.FindAll(doc, func(n *html.Node) bool {
		return internals.IsElement(n, "a") && strings.Contains(internals.Attr(n, "class"), "prevnextbutton")
	})

	if len(paginationLinks) == 2 {
		nextA := paginationLinks[1]

		nextHref := internals.Attr(nextA, "href")
		nextClass := internals.Attr(nextA, "class")
		if strings.Contains(nextClass, "disabled") || nextHref == "" || nextHref == "#" {
			fmt.Println("Reached last page")
			return pageResult.Devices, ""
		}
		return pageResult.Devices, baseURL + nextHref
	} else {
		return pageResult.Devices, ""
	}
}

func unique(input []string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, v := range input {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}
