package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"

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
	var scrapedDataFromName []byte
	var scrapedDataFromLink []byte
	if link != "" && brand != "none" && item != "none" && !strings.Contains(link, brand) {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "params error", "you might have entered wrong link")
		return
	}
	if link != "" && strings.Contains(link, "phonedb") {
		scrapedDataFromLink = fetchDataFromLink(link, source, item, brand, ctx)
		if scrapedDataFromLink == nil {
			log.Println("broken link provided")
		}
	}
	if item == "none" || brand == "none" {
		scrapedDataFromName = []byte{}
	} else {
		scrapedDataFromName, err = internals.FetchDataGSM(item, brand)
	}
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "process error", "failed scrapping data")
		return
	}

	var parsedDataFromName map[string]any
	if err := json.Unmarshal(scrapedDataFromName, &parsedDataFromName); len(scrapedDataFromName) > 0 && err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
		return
	}
	var parsedDataFromLink map[string]any
	if err := json.Unmarshal(scrapedDataFromLink, &parsedDataFromLink); len(scrapedDataFromLink) > 0 && link != "" && err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to extract data from provided link")
		return
	}
	finalDeviceData := combinedDeviceDataResponseGenerator(parsedDataFromName, parsedDataFromLink)

	phoneData := models.ScrapData{
		DeviceId:    uuid.New().String(),
		DeviceName:  item,
		DeviceBrand: brand,
		DeviceInfo:  finalDeviceData,
	}
	phoneData.Path = string(ctx.Path())
	response, err := json.Marshal(phoneData)
	if item == "none" || brand == "none" {
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBody(response)
		return
	}
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

	if err != nil {
		utils.SendError(ctx, fasthttp.StatusInternalServerError, "json error", "failed to generate response")
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetBody(response)
}

func (h *HandlerDb) UpdateDevices(ctx *fasthttp.RequestCtx) {
	manufacturerVal := ctx.UserValue("manufacturer_name")
	manufacturerEncoded, ok := manufacturerVal.(string)
	if !ok || manufacturerEncoded == "" {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "source type not found")
		return
	}
	manufacturer, err := url.PathUnescape(manufacturerEncoded)
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "invalid params", "source type invalid")
		return
	}

	result, err := h.Db.Query(context.TODO(), &dynamodb.QueryInput{
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

	if result == nil || len(result.Items) < 1 {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "data not found", "no such brand exist in database which you can update, please fetch the brand to automatically insert it in db")
	}

	brandURL, err := internals.FindBrandUrl(manufacturer)
	if err != nil {
		log.Println(err)
		return
	}

	t := unique(findDevices(brandURL))

	var newDevicesList models.DevicesInfo
	newDevicesList.BrandName = manufacturer
	newDevicesList.BrandDevicesList = t

	fmt.Println(len(t), result.Items)

	newList, err := attributevalue.Marshal(newDevicesList.BrandDevicesList)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		return
	}
	_, err = h.Db.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String("DevicesList"),
		Key: map[string]types.AttributeValue{
			"brandName": &types.AttributeValueMemberS{Value: manufacturer},
		},
		UpdateExpression: aws.String("set brandDeviceLists = :d"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":d": newList,
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.WriteString(`{"error": "` + err.Error() + `"}`)
		return
	}
	ctx.SetContentType("application/json")
	json.NewEncoder(ctx).Encode(newDevicesList)
}

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

func fetchDataFromLink(link string, source string, item string, brand string, ctx *fasthttp.RequestCtx) []byte {
	var scrapedData []byte
	link, err := url.PathUnescape(link)
	if err != nil {
		utils.SendError(ctx, fasthttp.StatusBadRequest, "params error", "invalid link provided")
		return nil
	}
	if source == "gsmarena" {
		scrapedData, err = internals.PrintSpecList(link, item, brand)
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "process error", "failed scrapping data gsm")
			return nil
		}
	}
	if source == "phonedb" {
		html, err := internals.FetchDetailTable(internals.PhoneResult{Title: item, DetailHref: link})
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "process error", "failed fetching data pdb")
			return nil
		}
		temp := internals.BodyObj{HtmlString: html, PhoneName: item, CompanyName: brand}
		scrapedData, err = internals.PDBParser(temp)
		if err != nil {
			utils.SendError(ctx, fasthttp.StatusInternalServerError, "process error", "failed scrapping data pdb")
			return nil
		}
	}
	return scrapedData
}

func combinedDeviceDataResponseGenerator(dataObj map[string]any, linkObj map[string]any) map[string]any {
	Final := make(map[string]any)

	extractData := func(obj map[string]any, keys []string) map[string]any {
		data := make(map[string]any)
		for _, v := range keys {
			val, ok := obj[v]
			if ok {
				data[v] = val
			}
		}
		return data
	}

	appender := func(obj map[string]any, parent string) {
		for k, v := range obj {
			Final["deviceData_"+parent+"_"+k] = v
		}
	}

	keyChecker := func(j map[string]any, s string) bool {
		_, ok := j[s]
		return ok
	}

	linkDataCheckerAndAppender := func(obj map[string]any, s string) {
		switch s {
		case "Introduction":
			if val, ok := obj["Codename"]; ok {
				Final["deviceData_Launch_Codename"] = val
			}
			if val, ok := obj["Announced"]; ok && !keyChecker(Final, "deviceData_Launch_Announced") {
				Final["deviceData_Launch_Announced"] = val
			}
			if val, ok := obj["Released"]; ok && !keyChecker(Final, "deviceData_Launch_Status") {
				Final["deviceData_Launch_Status"] = val
			}

		case "Physical Attributes":
			if !keyChecker(Final, "deviceData_Body_Dimensions") {
				dimens := ""
				if val, ok := obj["Height"]; ok {
					dimens = val.(string)
				}
				if val, ok := obj["Width"]; ok {
					dimens += " x " + val.(string)
				}
				if val, ok := obj["Depth"]; ok {
					dimens += " x " + val.(string)
				}
				if dimens != "" {
					Final["deviceData_Body_Dimensions"] = dimens
				}
			}
			if val, ok := obj["Mass"]; ok && !keyChecker(Final, "deviceData_Body_Weight") {
				Final["deviceData_Body_Weight"] = val
			}

		case "Software Environment":
			if val, ok := obj["Platform"]; ok && !keyChecker(Final, "deviceData_Platform_OS") {
				Final["deviceData_Platform_OS"] = val
			}

		case "Display":
			if !keyChecker(Final, "deviceData_Display_Size") {
				size := ""
				if val, ok := obj["Display Diagonal"]; ok {
					size = val.(string)
				}
				if val, ok := obj["Display Area Utilization"]; ok && size != "" {
					size += " (~" + val.(string) + " screen-to-body ratio)"
				}
				if size != "" {
					Final["deviceData_Display_Size"] = size
				}
			}
			if !keyChecker(Final, "deviceData_Display_Resolution") {
				res := ""
				if val, ok := obj["Resolution"]; ok {
					res = val.(string) + " pixels, "
				}
				if val, ok := obj["Pixel Density"]; ok && res != "" {
					res += val.(string) + " ppi density"
				}
				if res != "" {
					Final["deviceData_Display_Resolution"] = res
				}
			}
			dimens := ""
			if val, ok := obj["Display Height"]; ok {
				dimens = val.(string)
			}
			if val, ok := obj["Display Width"]; ok && dimens != "" {
				dimens += " x " + val.(string)
			}
			if dimens != "" {
				Final["deviceData_Display_Dimensions"] = dimens
			}
			if val, ok := obj["Display Refresh Rate"]; ok {
				Final["deviceData_Refresh_Rate"] = val
			}

		case "Cellular Phone":
			if val, ok := obj["SIM Card Slot"]; ok && !keyChecker(Final, "deviceData_Memory_Card slot") {
				Final["deviceData_Memory_Card slot"] = val
			}

		case "Communication Interfaces":
			if val, ok := obj["Max. Charging Power"]; ok && !keyChecker(Final, "deviceData_Battery_Charging") {
				Final["deviceData_Battery_Charging"] = val
			}
			if val, ok := obj["USB"]; ok {
				Final["deviceData_Comms_USB"] = val
			}
			if val, ok := obj["Bluetooth"]; ok && !keyChecker(Final, "deviceData_Comms_Bluetooth") {
				Final["deviceData_Comms_Bluetooth"] = val
			}

		case "Power Supply":
			if !keyChecker(Final, "deviceData_Battery_Type") {
				btype := ""
				if val, ok := obj["Battery"]; ok {
					btype = val.(string)
				}
				if val, ok := obj["Nominal Battery Capacity"]; ok {
					btype += " " + val.(string)
				}
				if btype != "" {
					Final["deviceData_Battery_Type"] = btype
				}
			}

			if val, ok := obj["Max. Wireless Charging Power"]; ok {
				Final["deviceData_Battery_WirelessCharging_Info"] = val
			}
			if val, ok := obj["Wireless Charging"]; ok {
				Final["deviceData_Battery_WirelessCharging"] = val
			}

		case "Geographical Attributes":
			if val, ok := obj["Market Countries"]; ok {
				Final["deviceData_Market_Countries"] = val
			}
			if val, ok := obj["Market Regions"]; ok {
				Final["deviceData_Market_Regions"] = val
			}
			if val, ok := obj["Mobile Operator"]; ok {
				Final["deviceData_Mobile_Operator"] = val
			}

		default:
			fmt.Println("provided key was not required:", s)
		}
	}

	if len(linkObj) == 0 && len(dataObj) == 0 {
		return nil
	}

	if len(dataObj) > 0 {
		for subk1, subv1 := range dataObj {
			nested, ok := subv1.(map[string]any)
			if !ok {
				fmt.Println(subk1, "value is not a map, skipping")
				continue
			}
			switch subk1 {
			case "Launch":
				appender(extractData(nested, []string{"Announced", "Status"}), subk1)
			case "Body":
				appender(extractData(nested, []string{"Dimensions", "Weight", "SIM"}), subk1)
			case "Display":
				appender(extractData(nested, []string{"Type", "Size", "Resolution"}), subk1)
			case "Platform":
				appender(extractData(nested, []string{"OS", "Chipset"}), subk1)
			case "Memory":
				appender(extractData(nested, []string{"Card slot", "Internal"}), subk1)
			case "Sound":
				appender(extractData(nested, []string{"Loudspeaker", "3.5mm jack"}), subk1)
			case "Battery":
				appender(extractData(nested, []string{"Charging", "Type"}), subk1)
			case "Misc":
				appender(extractData(nested, []string{"Colors", "Models"}), subk1)
			case "Comms":
				appender(extractData(nested, []string{"Bluetooth", "WLAN", "USB"}), subk1)
			default:
				fmt.Println(subk1, "was not required")
			}
		}
	}

	if len(linkObj) > 0 {
		if info, ok := linkObj["info"]; ok {
			infoMap, ok := info.(map[string]any)
			if !ok {
				fmt.Println("linkObj 'info' is not a map[string]any")
				return Final
			}
			for subk1, subv1 := range infoMap {
				nested, ok := subv1.(map[string]any)
				if !ok {
					fmt.Println(subk1, "value is not a map, skipping")
					continue
				}
				switch subk1 {
				case "Introduction":
					linkDataCheckerAndAppender(extractData(nested, []string{"Codename", "Announced", "Released"}), subk1)
				case "Physical Attributes":
					linkDataCheckerAndAppender(extractData(nested, []string{"Height", "Width", "Depth", "Mass"}), subk1)
				case "Software Environment":
					linkDataCheckerAndAppender(extractData(nested, []string{"Platform"}), subk1)
				case "Display":
					linkDataCheckerAndAppender(extractData(nested, []string{"Display Diagonal", "Display Width", "Display Height", "Pixel Density", "Resolution", "Display Refresh Rate", "Display Area Utilization"}), subk1)
				case "Cellular Phone":
					linkDataCheckerAndAppender(extractData(nested, []string{"SIM Card Slot"}), subk1)
				case "Communication Interfaces":
					linkDataCheckerAndAppender(extractData(nested, []string{"Max. Charging Power", "USB", "Bluetooth"}), subk1)
				case "Power Supply":
					linkDataCheckerAndAppender(extractData(nested, []string{"Battery", "Max. Wireless Charging Power", "Wireless Charging", "Nominal Battery Capacity"}), subk1)
				case "Geographical Attributes":
					linkDataCheckerAndAppender(extractData(nested, []string{"Market Countries", "Market Regions", "Mobile Operator"}), subk1)
				default:
					fmt.Println(subk1, "was not required")
				}
			}
		}
	}

	return Final
}
