package internals

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	baseURL   = "https://www.gsmarena.com/"
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var client = &http.Client{Timeout: 20 * time.Second}

func FetchHTML(url string) (*html.Node, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", baseURL)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return html.Parse(strings.NewReader(string(body)))
}

func Attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func HasClass(n *html.Node, cls string) bool {
	classes := strings.Fields(Attr(n, "class"))
	for _, c := range classes {
		if c == cls {
			return true
		}
	}
	return false
}

func TextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var parts []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		t := TextContent(c)
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

func walk(n *html.Node, fn func(*html.Node) bool) {
	if !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func FindAll(root *html.Node, match func(*html.Node) bool) []*html.Node {
	var results []*html.Node
	walk(root, func(n *html.Node) bool {
		if match(n) {
			results = append(results, n)
		}
		return true
	})
	return results
}

func FindFirst(root *html.Node, match func(*html.Node) bool) *html.Node {
	var result *html.Node
	walk(root, func(n *html.Node) bool {
		if result != nil {
			return false
		}
		if match(n) {
			result = n
			return false
		}
		return true
	})
	return result
}

func IsElement(n *html.Node, tag string) bool {
	return n.Type == html.ElementNode && n.Data == tag
}

func FindBrandUrl(brandName string) (string, error) {
	doc, err := FetchHTML(baseURL + "makers.php3")
	if err != nil {
		return "", fmt.Errorf("fetch makers page: %w", err)
	}

	target := strings.ToLower(strings.TrimSpace(brandName))

	links := FindAll(doc, func(n *html.Node) bool {
		return IsElement(n, "a") && strings.Contains(Attr(n, "href"), "-phones-")
	})

	for _, a := range links {
		text := strings.ToLower(strings.TrimSpace(TextContent(a)))

		innerSpan := FindFirst(a, func(n *html.Node) bool {
			return IsElement(n, "span")
		})
		if innerSpan != nil {
			spanText := TextContent(innerSpan)
			text = strings.ToLower(strings.TrimSpace(strings.Replace(TextContent(a), spanText, "", 1)))
		}
		text = strings.TrimSpace(text)

		if text == target {
			href := Attr(a, "href")
			return baseURL + href, nil
		}
	}

	return "", fmt.Errorf("brand %q not found on makers page", brandName)
}

type pageResult struct {
	DeviceURL string
	NextURL   string
}

func ScanPage(doc *html.Node, deviceName string) pageResult {
	target := strings.ToLower(strings.TrimSpace(deviceName))

	makersDiv := FindFirst(doc, func(n *html.Node) bool {
		return IsElement(n, "div") && HasClass(n, "makers")
	})
	if makersDiv == nil {
		return pageResult{}
	}

	items := FindAll(makersDiv, func(n *html.Node) bool {
		return IsElement(n, "li")
	})

	for _, li := range items {
		a := FindFirst(li, func(n *html.Node) bool { return IsElement(n, "a") })
		if a == nil {
			continue
		}
		span := FindFirst(a, func(n *html.Node) bool { return IsElement(n, "span") })
		if span == nil {
			continue
		}
		name := strings.TrimSpace(TextContent(span))
		// if strings.ToLower(name) == target {
		nameClean := strings.ToLower(strings.TrimSpace(name))
		targetClean := strings.ToLower(strings.TrimSpace(target))

		if strings.Contains(nameClean, targetClean) {
			href := Attr(a, "href")
			return pageResult{DeviceURL: baseURL + href}
		}
	}

	paginationLinks := FindAll(doc, func(n *html.Node) bool {
		return IsElement(n, "a") && strings.Contains(Attr(n, "class"), "prevnextbutton")
	})

	if len(paginationLinks) == 2 {
		nextA := paginationLinks[1]

		nextHref := Attr(nextA, "href")
		nextClass := Attr(nextA, "class")
		if strings.Contains(nextClass, "disabled") || nextHref == "" || nextHref == "#" {
			fmt.Println("Reached last page, device not found.")
			return pageResult{}
		}
		return pageResult{NextURL: baseURL + nextHref}
	}
	return pageResult{}
}

func findDeviceURL(brandURL, deviceName string) (string, error) {
	currentURL := brandURL
	page := 1

	for {
		doc, err := FetchHTML(currentURL)
		if err != nil {
			return "", fmt.Errorf("fetch brand page: %w", err)
		}

		result := ScanPage(doc, deviceName)

		if result.DeviceURL != "" {
			return result.DeviceURL, nil
		}
		if result.NextURL != "" {
			currentURL = result.NextURL
			page++
			time.Sleep(1000 * time.Millisecond)
			continue
		}

		break
	}

	return "", fmt.Errorf("device %q not found under brand", deviceName)
}

func FindBrandsInGSM() ([]string, error) {
	doc, err := FetchHTML(baseURL + "makers.php3")
	if err != nil {
		return []string{}, fmt.Errorf("fetch makers page: %w", err)
	}

	links := FindAll(doc, func(n *html.Node) bool {
		return IsElement(n, "a") && strings.Contains(Attr(n, "href"), "-phones-")
	})
	var res []string
	for _, a := range links {
		text := strings.ToLower(strings.TrimSpace(TextContent(a)))

		innerSpan := FindFirst(a, func(n *html.Node) bool {
			return IsElement(n, "span")
		})
		if innerSpan != nil {
			spanText := TextContent(innerSpan)
			text = strings.ToLower(strings.TrimSpace(strings.Replace(TextContent(a), spanText, "", 1)))
		}
		text = strings.TrimSpace(text)

		if text != "" {
			res = append(res, text)
		}
	}

	if len(res) == 0 {
		return []string{}, fmt.Errorf("brands not found")
	}
	return res, nil
}

func renderNode(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		return
	}
	if n.Type != html.ElementNode {
		return
	}

	sb.WriteString("<" + n.Data)
	for _, a := range n.Attr {
		sb.WriteString(fmt.Sprintf(` %s="%s"`, a.Key, a.Val))
	}
	sb.WriteString(">")

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderNode(c, sb)
	}

	sb.WriteString("</" + n.Data + ">")
}

func PrintSpecList(deviceURL string, device string, brand string) ([]byte, error) {
	doc, err := FetchHTML(deviceURL)
	if err != nil {
		return nil, fmt.Errorf("fetch device page: %w", err)
	}

	div := FindFirst(doc, func(n *html.Node) bool {
		return IsElement(n, "div") && Attr(n, "id") == "specs-list"
	})

	if div == nil {
		return nil, fmt.Errorf("no table found on device page")
	}

	var sb strings.Builder
	renderNode(div, &sb)

	body, _ := json.Marshal(Body{
		HtmlString:  sb.String(),
		PhoneName:   device,
		CompanyName: brand,
	})

	parsed := GsmParser(body)
	return parsed, nil
}

func FetchDataGSM(device string, brand string) ([]byte, error) {

	brandName := brand
	if brandName == "" {
		brandName = "Google"
	}
	deviceName := device
	if deviceName == "" {
		deviceName = "Pixel 10 Pro"
	}

	brandURL, err := FindBrandUrl(brandName)
	if err != nil {
		fmt.Println(err)
	}
	deviceURL, err := findDeviceURL(brandURL, deviceName)

	if deviceURL == "" {
		phoneDbResult, doDeviceExistInPhoneDb := ScrapeFirstPhone(deviceName)
		if doDeviceExistInPhoneDb {
			html, err := FetchDetailTable(phoneDbResult)
			if err != nil {
				log.Fatal(err)
			}
			temp := BodyObj{HtmlString: html, PhoneName: deviceName, CompanyName: brandName}
			result, err := PDBParser(temp)
			if err != nil {
				log.Fatal(err)
			}
			if err == nil {
				return result, nil
			}
		}
	} else {
		return PrintSpecList(deviceURL, deviceName, brandName)
	}
	return []byte{}, nil
}

type Body struct {
	HtmlString  string `json:"html"`
	PhoneName   string `json:"phone"`
	CompanyName string `json:"company"`
}

// func GsmParser(object []byte) []byte {
// 	var body Body

// 	if err := json.Unmarshal(object, &body); err != nil {
// 		log.Fatal("some issue occured while unmarshaling")
// 		return nil
// 	}

// 	doc, _ := html.Parse(strings.NewReader(body.HtmlString))
// 	var pointer *html.Node
// 	var multipleData []string
// 	var temp = JsonObject{}
// 	var extractJson func(*html.Node)
// 	var category, topic string
// 	occurance := 0
// 	isOtherKey := false
// 	extractJson = func(n *html.Node) {
// 		if n.Type == html.ElementNode {
// 			if n.Data == "th" || n.Data == "td" {
// 				pointer = n
// 			}
// 			if n.Data == "tr" {
// 				occurance = 0
// 			}
// 		}
// 		if n.Type == html.TextNode && pointer != nil && n.NextSibling != nil && n.NextSibling.Data == "hr" {
// 			multipleData = append(multipleData, n.Data)
// 		} else if n.Type == html.TextNode && pointer != nil && n.PrevSibling != nil && n.PrevSibling.Data == "hr" {
// 			multipleData = append(multipleData, n.Data)
// 		} else if n.Type == html.TextNode && pointer != nil {
// 			cleanText := strings.TrimSpace(n.Data)
// 			if cleanText != "" {
// 				if pointer.Data == "th" {
// 					category = cleanText
// 					if _, exists := temp[category]; !exists {
// 						temp[category] = make(JsonObject)
// 					}
// 				} else if pointer.Data == "td" && category != "" {
// 					if isOtherKey {
// 						for _, a := range pointer.Attr {
// 							if a.Key == "class" && strings.Contains(a.Val, "ttl") {
// 								isOtherKey = !isOtherKey
// 								occurance++
// 								switch occurance {
// 								case 1:
// 									topic = cleanText
// 								case 2:
// 									if inner, ok := temp[category].(JsonObject); ok {
// 										inner[topic] = cleanText
// 									}
// 									occurance = 0
// 								}
// 							}
// 						}
// 						multipleData = append(multipleData, pointer.Data)
// 					} else {
// 						occurance++
// 						switch occurance {
// 						case 1:
// 							topic = cleanText
// 						case 2:
// 							if inner, ok := temp[category].(JsonObject); ok {
// 								inner[topic] = cleanText
// 							}
// 							occurance = 0
// 						}
// 					}
// 				}
// 			} else {
// 				for _, a := range pointer.Attr {
// 					if a.Key == "class" && strings.Contains(a.Val, "ttl") {
// 						isOtherKey = !isOtherKey
// 					}
// 				}
// 			}
// 		}

// 		for c := n.FirstChild; c != nil; c = c.NextSibling {
// 			extractJson(c)
// 		}
// 		if n == pointer {
// 			if len(multipleData) > 0 {
// 				if inner, ok := temp[category].(JsonObject); ok {
// 					inner["OtherInfo"] = multipleData
// 					multipleData = []string{}
// 				}
// 			}
// 			pointer = nil
// 		}
// 	}
// 	extractJson(doc.FirstChild)

// 	tempJSON, _ := json.MarshalIndent(temp, "", "  ")
// 	return tempJSON
// }

func GsmParser(object []byte) []byte {
	var body Body

	if err := json.Unmarshal(object, &body); err != nil {
		log.Fatal("unmarshal error:", err)
		return nil
	}

	doc, _ := html.Parse(strings.NewReader(body.HtmlString))

	temp := JsonObject{}
	var category string
	getText := func(n *html.Node) string {
		var result []string
		var f func(*html.Node)
		f = func(n *html.Node) {
			if n.Type == html.TextNode {
				txt := strings.TrimSpace(strings.ReplaceAll(n.Data, "\u00a0", ""))
				if txt != "" {
					result = append(result, txt)
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
		f(n)
		return strings.Join(result, " ")
	}

	getParts := func(n *html.Node) []string {
		var parts []string
		var current []string
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "hr" {
				if len(current) > 0 {
					parts = append(parts, strings.Join(current, " "))
					current = []string{}
				}
				continue
			}
			if c.Type == html.TextNode {
				txt := strings.TrimSpace(strings.ReplaceAll(c.Data, "\u00a0", ""))
				if txt != "" {
					current = append(current, txt)
				}
			}
		}
		if len(current) > 0 {
			parts = append(parts, strings.Join(current, " "))
		}
		return parts
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var th *html.Node
			var tds []*html.Node
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode {
					if c.Data == "th" {
						th = c
					} else if c.Data == "td" {
						tds = append(tds, c)
					}
				}
			}
			if th != nil {
				category = getText(th)
				if _, ok := temp[category]; !ok {
					temp[category] = make(JsonObject)
				}
			}
			if len(tds) < 2 || category == "" {
				goto NEXT
			}
			keyText := getText(tds[0])
			if keyText == "" {
				parts := getParts(tds[1])
				if len(parts) > 0 {
					if inner, ok := temp[category].(JsonObject); ok {
						inner["OtherInfo"] = parts
					}
				}
			} else {
				value := getText(tds[1])
				if inner, ok := temp[category].(JsonObject); ok {
					inner[keyText] = value
				}
			}

		}
	NEXT:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	tempJSON, _ := json.MarshalIndent(temp, "", "  ")
	return tempJSON
}
