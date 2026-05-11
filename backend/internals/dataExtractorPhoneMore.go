package internals

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"golang.org/x/net/html"
)

func FetchDeviceFromPhoneMore(brandName string, deviceName string) (error, string) {
	c := colly.NewCollector()
	c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
	c.WithTransport(&http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	})

	htmlExtracted := ""
	c.OnRequest(func(r *colly.Request) {
		log.Println("Visiting:", r.URL.String())
	})

	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("ERR:", err)
	})
	found := false
	c.OnHTML("table", func(e *colly.HTMLElement) {
		if found {
			return
		}
		found = true
		html, err := goquery.OuterHtml(e.DOM)
		if err != nil {
			fmt.Println("Error getting outerHTML:", err)
			return
		}
		htmlExtracted = html
	})
	deviceArr := strings.Split(deviceName, " ")
	deviceStr := ""
	for i, v := range deviceArr {
		if i == len(deviceArr)-1 {
			deviceStr += strings.ToLower(v)
		} else {
			deviceStr += strings.ToLower(v) + "-"
		}
	}
	err := c.Visit("https://www.phonemore.com/specs/" + strings.ToLower(brandName) + "/" + deviceStr + "/")
	if err != nil {
		return err, ""
	}

	c.Wait()

	return nil, htmlExtracted
}

func FetchHTMLDataPhoneMore(htmlstr string) ([]byte, error) {
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

	var tempArr []string
	var tempstr string
	occurance := 0
	var parseSecondTd func(*html.Node, bool) []string
	parseSecondTd = func(n *html.Node, root bool) []string {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "br" || c.Data == "hr") {
				if strings.TrimSpace(tempstr) != "" {
					tempArr = append(tempArr, strings.TrimSpace(tempstr))
				}
				tempstr = ""
				occurance++
				continue
			}
			if c.Type == html.TextNode {
				if c.Data[0] == ',' {
					tempstr += c.Data[1:] + ", "
				} else {
					tempstr += c.Data + ", "
				}
				continue
			}
			if c.Type == html.ElementNode {
				parseSecondTd(c, false)
			}
		}
		if root && strings.TrimSpace(tempstr) != "" {
			tempArr = append(tempArr, strings.TrimSpace(tempstr))
		}
		return tempArr
	}

	parseTable := func(n *html.Node) {
		if n.Data != "tr" {
			return
		}
		var key string
		var val []string
		tdIndex := 0
		for td := n.FirstChild; td != nil; td = td.NextSibling {
			if td.Data != "td" {
				continue
			}
			if td.FirstChild == nil {
				return
			}
			switch tdIndex {
			case 0:
				key = parseFirstTd(td)
			case 1:
				tempArr = []string{}
				tempstr = ""
				occurance = 0
				val = parseSecondTd(td, true)
				if val == nil {
					val = []string{}
				}
			}
			tdIndex++
		}
		if key != "" && len(val) > 0 {
			v, ok := temp[category].(JsonObject)
			if !ok {
				v = JsonObject{}
				temp[category] = v
			}
			v[key] = val
		}
	}

	var walk func(*html.Node)
	mainTableContainerParsed := false
	walk = func(n *html.Node) {
		if n.Data == "table" && Attr(n, "id") == "tb_specs" {
			mainTableContainerParsed = true
			for c := n.FirstChild.FirstChild; c != nil; c = c.NextSibling {
				if c.Data == "tr" && c.FirstChild.Data == "td" && c.FirstChild.NextSibling == nil {
					if c.FirstChild.FirstChild.Data == "h2" && c.FirstChild.FirstChild.FirstChild.Type == html.TextNode {
						category = c.FirstChild.FirstChild.FirstChild.Data
					}
				} else {
					if c.Data == "tr" {
						parseTable(c)
					}
				}
			}
		}

		if !mainTableContainerParsed {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(doc)
	tempJSON, _ := json.MarshalIndent(temp, "", "  ")
	return tempJSON, nil
}
