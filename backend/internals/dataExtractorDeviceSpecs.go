package internals

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"golang.org/x/net/html"
)

func devicespecificationHtmlFetcher(url string) []byte {
	options := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(profiles.Chrome_103),
		tls_client.WithNotFollowRedirects(),
	}

	client, err := tls_client.NewHttpClient(
		tls_client.NewNoopLogger(),
		options...,
	)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	req.Header = http.Header{
		"accept": []string{
			"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		},
		"accept-language": []string{
			"en-US,en;q=0.9",
		},
		"user-agent": []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	return body
}

func confidence(s string) bool {
	var ignoredWords = []string{
		"5g",
		"4g",
		"3g",
		"2g",
		"lte",
		"volte",
		"uw",
		"uwb",
		"td-lte",
		"nr",
		"64gb",
		"128gb",
		"256gb",
		"512gb",
		"1tb",
		"2tb",
		"4gb",
		"6gb",
		"8gb",
		"12gb",
		"16gb",
		"sm-",
		"a",
		"global-rom",
		"nfc",
		"wifi",
		"bluetooth",
	}
	for _, v := range ignoredWords {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

func deviceNameChecker(websiteDeviceName []string, deviceName []string) bool {
	i, j := 0, 0

	for j < len(deviceName) {
		if i >= len(websiteDeviceName) {
			return false
		}
		if websiteDeviceName[i] != deviceName[j] {
			if !confidence(websiteDeviceName[i]) {
				return false
			}
			i++
			if i >= len(websiteDeviceName) {
				return false
			}
		}
		if websiteDeviceName[i] == deviceName[j] {
			i++
			j++
		}
	}
	if j == len(deviceName)-1 && i < len(websiteDeviceName) {
		return true
	}
	return true
}

func findDevice(n *html.Node, deviceFlag *bool, deviceName string) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Data == "div" && strings.Contains(Attr(c, "id"), "model_") && !*deviceFlag {
			aElem := c.FirstChild
			if aElem != nil && aElem.Data == "a" && aElem.NextSibling != nil && aElem.NextSibling.Data == "h3" {
				aInH3 := aElem.NextSibling.FirstChild
				if aInH3 != nil && deviceNameChecker(strings.Split(aInH3.FirstChild.Data, " "), strings.Split(deviceName, " ")) {
					return Attr(aElem, "href")
				}
			}
		}
	}
	return ""
}

func parseDevicePage(n *html.Node, mainDivContainerParsed *bool, flag *bool, name string) string {
	deviceLink := ""
	if n.Data == "div" && Attr(n, "id") == "main" {
		*mainDivContainerParsed = true
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Data == "div" && strings.Contains(Attr(c, "class"), "model-listing-container") && !*flag {
				deviceLink = findDevice(c, flag, name)
				if deviceLink != "" {
					return deviceLink
				}
			}
		}
	}

	if !*mainDivContainerParsed {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			str := parseDevicePage(c, mainDivContainerParsed, flag, name)
			if str != "" {
				return str
			}
		}
	}
	return deviceLink
}

func FetchDeviceFromDeviceSpecification(brandName string, deviceName string) (error, string) {

	body := devicespecificationHtmlFetcher("https://www.devicespecifications.com/en/brand-more")
	if body == nil {
		return errors.New("error occured while fetching data"), ""
	}
	doc, _ := html.Parse(strings.NewReader(string(body)))
	brandLink := ""
	var walk func(*html.Node)
	mainContainerParsed := false
	walk = func(n *html.Node) {
		if n.Data == "div" && Attr(n, "class") == "brand-listing-container-news" {
			mainContainerParsed = true
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.FirstChild != nil && c.FirstChild.Type == html.TextNode && strings.EqualFold(c.FirstChild.Data, brandName) {
					brandLink = Attr(c, "href")
				}
			}
		}

		if !mainContainerParsed {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(doc)

	deviceFlag := false
	deviceLink := ""
	mainDivContainerParsed := false

	if brandLink != "" {
		devicesBody := devicespecificationHtmlFetcher(brandLink)
		if devicesBody == nil {
			return errors.New("error occured while fetching data"), ""
		}
		deviceDoc, _ := html.Parse(strings.NewReader(string(devicesBody)))
		deviceLink = parseDevicePage(deviceDoc, &mainDivContainerParsed, &deviceFlag, deviceName)

	} else {
		return errors.New("no such brand found"), ""
	}

	if deviceLink != "" {
		devicesData := devicespecificationHtmlFetcher(deviceLink)
		if devicesData == nil {
			return errors.New("error occured while fetching data"), ""
		}
		return nil, string(devicesData)
	} else {
		return errors.New("no such device found"), ""
	}
}

func FetchDataDeviceSpecifications(htmlstr string) ([]byte, error) {
	doc, _ := html.Parse(strings.NewReader(htmlstr))
	temp := JsonObject{}

	var category string

	parseFirstTd := func(n *html.Node) string {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				return c.Data
			}
		}
		return ""
	}

	parseSecondTd := func(n *html.Node) []string {
		var temp []string
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			i := 0
			if c.Type == html.TextNode {
				temp = append(temp, c.Data)
				i++
			}
			if c.Type == html.ElementNode && c.Data == "span" {
				if c.FirstChild != nil && c.FirstChild.Type == html.TextNode && i > 0 {
					temp[i-1] = temp[i-1] + c.FirstChild.Data
				}
			}
		}
		return temp
	}

	parseTable := func(n *html.Node) {
		if category != "" {
			if n.FirstChild.Data == "tbody" {
				for tr := n.FirstChild.FirstChild; tr != nil; tr = tr.NextSibling {
					firstTD := false
					var key string
					var val []string
					if tr.Type == html.ElementNode && tr.Data == "tr" {
						for td := tr.FirstChild; td != nil; td = td.NextSibling {
							if td.Data == "td" {
								if !firstTD {
									firstTD = true
									key = parseFirstTd(td)
									if key == "" {
										key = "otherInfo"
									}
								} else {
									val = parseSecondTd(td)
								}
							}
						}
						if val == nil {
							val = make([]string, 0)
						}
						if v, ok := temp[category].(JsonObject); ok {
							v[key] = val
						}
					}
				}
			}
		}
	}

	parseHeader := func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "h2" {
				if c.FirstChild.Type == html.TextNode {
					category = c.FirstChild.Data
				}
			}
			if category != "" {
				if _, exists := temp[category]; !exists {
					temp[category] = make(JsonObject)
				}
			}
		}
	}

	var walk func(*html.Node)
	mainDivContainerParsed := false
	walk = func(n *html.Node) {
		if n.FirstChild != nil && n.FirstChild.Data == "header" && Attr(n.FirstChild, "class") == "section-header" {
			mainDivContainerParsed = true
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Data == "header" && Attr(c, "class") == "section-header" {
					parseHeader(c)
				}
				if c.Data == "table" && Attr(c, "class") == "model-information-table row-selection" {
					parseTable(c)
				}
			}
		}

		if !mainDivContainerParsed {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(doc)
	tempJSON, _ := json.MarshalIndent(temp, "", "  ")
	return tempJSON, nil
}
