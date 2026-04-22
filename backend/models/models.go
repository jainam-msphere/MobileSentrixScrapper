package models

type ScrapData struct {
	Path        string         `json:"path"`
	DeviceBrand string         `json:"deviceBrand" dynamodbav:"deviceBrand"`
	DeviceName  string         `json:"deviceName" dynamodbav:"deviceName"`
	DeviceId    string         `json:"deviceId" dynamodbav:"deviceId"`
	DeviceInfo  map[string]any `json:"deviceInfo" dynamodbav:"deviceInfo"`
}

type DevicesInfo struct {
	BrandName        string   `dynamodbav:"brandName"`
	BrandDevicesList []string `dynamodbav:"brandDeviceLists"`
}

type BrandsInfo struct {
	KeyName   string   `dynamodbav:"keyName"`
	BrandList []string `dynamodbav:"brandLists"`
}
