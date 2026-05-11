package internals

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"golang.org/x/net/html"
)

type PhoneResult struct {
	Title      string
	DetailHref string
}

func ScrapeFirstPhone(phone string) (PhoneResult, bool) {
	var match PhoneResult
	found := false

	collyCollector := colly.NewCollector(
		colly.AllowedDomains("phonedb.net", "www.phonedb.net"),
		colly.UserAgent(
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) "+
				"Chrome/124.0.0.0 Safari/537.36",
		),
	)
	collyCollector.WithTransport(&http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	})

	collyCollector.OnRequest(func(r *colly.Request) {
		fmt.Printf("→ Search POST: %s\n", r.URL)
	})
	collyCollector.OnResponse(func(r *colly.Response) {
		fmt.Printf("← Response: %d (%d bytes\n", r.StatusCode, len(r.Body))
		err := os.WriteFile("./htm.txt", r.Body, 0644)
		if err != nil {
			panic(fmt.Errorf("could not write file %s:", err))
		}
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(string(r.Body)))
		root := doc.Selection
		res, ok := parsePhoneBlocks(root, phone)
		fmt.Println(res, ok)
		if r.StatusCode == 200 && ok {
			match = res
			found = true
		}
	})
	collyCollector.OnError(func(r *colly.Response, err error) {
		log.Printf("Error [%s]: %v", r.Request.URL, err)
	})

	err := collyCollector.Post("https://phonedb.net/index.php?m=device&s=list", map[string]string{
		"search_exp": phone,
	})
	if err != nil {
		log.Fatalf("POST failed: %v", err)
	}

	return match, found
}

func FetchDetailTablePhoneDb(result PhoneResult) (string, error) {
	collyCollector := colly.NewCollector(
		colly.AllowedDomains("phonedb.net", "www.phonedb.net"),
		colly.UserAgent(
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) "+
				"Chrome/124.0.0.0 Safari/537.36",
		),
	)
	collyCollector.WithTransport(&http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	})
	var tableHTML string
	var fetchErr error
	tableFound := false
	collyCollector.OnHTML("a[title='Detailed view contains even more details']", func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr("href"))
		e.Request.Visit(link)
	})
	collyCollector.OnHTML("div.container div.canvas table", func(table *colly.HTMLElement) {
		if tableFound {
			return
		}
		tableFound = true

		html, err := goquery.OuterHtml(table.DOM)
		if err != nil {
			fetchErr = err
			return
		}
		tableHTML = html
	})

	collyCollector.OnError(func(r *colly.Response, err error) {
		fetchErr = err
	})

	err := collyCollector.Visit(result.DetailHref)
	if err != nil {
		return "", err
	}

	if fetchErr != nil {
		return "", fetchErr
	}

	if !tableFound {
		return "", fmt.Errorf("no table found at div.container div.canvas table")
	}
	return tableHTML, nil
}

func parsePhoneBlocks(container *goquery.Selection, phone string) (PhoneResult, bool) {
	var match PhoneResult

	container.Find("div.content_block").EachWithBreak(func(_ int, block *goquery.Selection) bool {

		titleDiv := block.Find("div.content_block_title")
		if titleDiv.Length() == 0 {
			return true
		}

		titleText := strings.TrimSpace(titleDiv.Find("a").First().Text())

		if !strings.Contains(strings.ToLower(titleText), strings.ToLower(phone)) {
			return true
		}

		var detailHref string
		block.Find("a").Each(func(_ int, a *goquery.Selection) {
			if strings.EqualFold(strings.TrimSpace(a.Text()), "all details") {
				if href, exists := a.Attr("href"); exists {
					detailHref = href
				}
			}
		})

		if detailHref == "" {
			return true
		}

		match = PhoneResult{
			Title:      titleText,
			DetailHref: detailHref,
		}

		return false
	})

	if match.Title != "" {
		return match, true
	}

	return PhoneResult{}, false
}

type JsonObject map[string]any

type BodyObj struct {
	HtmlString  string `json:"html"`
	PhoneName   string `json:"phone"`
	CompanyName string `json:"company"`
}

func PDBParser(body BodyObj) ([]byte, error) {
	doc, err := html.Parse(strings.NewReader(body.HtmlString))
	if err != nil {
		return []byte{}, err
	}

	info := JsonObject{}
	currentSection := ""
	currentSectionExtra := false

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			tds := getTD(n)
			if len(tds) == 0 {
				return
			}
			isColspanExist := false
			for _, attr := range tds[0].Attr {
				if attr.Key == "colspan" {
					isColspanExist = true
				}
			}
			switch {
			case len(tds) == 1 || (len(tds) > 0 && isColspanExist):
				var heading string
				var key, value string
				occurance := 0
				var findTextInH4H5Node func(*html.Node)
				findTextInH4H5Node = func(node *html.Node) {
					if node.Type == html.ElementNode && (node.Data == "h4" || node.Data == "h5") {
						texts := extractText(node)
						result := strings.TrimSpace(strings.Join(texts, " "))
						result = strings.TrimSpace(strings.TrimSuffix(result, ":"))
						heading = result
						if heading != "" {
							currentSection = heading
							currentSectionExtra = false
							if _, exists := info[currentSection]; !exists {
								info[currentSection] = JsonObject{}
							}
						}
					}
					if node.Type == html.TextNode && node.Parent.Data == "strong" {
						texts := node.Data
						result := strings.TrimSpace(strings.TrimSuffix(texts, ":"))
						key = result
						occurance++
					}
					if node.Type == html.TextNode && occurance == 1 && node.Parent.Data != "strong" {
						value = node.Data
						occurance++
					}
					if occurance == 2 {
						if currentSection == "" {
							info[key] = value
						} else {
							if key != "" && key != "extras" {
								if inner, ok := info[currentSection].(JsonObject); ok {
									inner[key] = value
								}
							} else if key == "extras" {
								if inner, ok := info[currentSection].(JsonObject); ok {
									inner["extras"] = value
								}
							}
						}
						occurance = 0
					}
					for c := node.FirstChild; c != nil; c = c.NextSibling {
						findTextInH4H5Node(c)
					}
				}
				findTextInH4H5Node(tds[0])
				if heading == "" {
					texts := extractText(tds[0])
					heading = strings.TrimSuffix(strings.Join(texts, " "), "")
					heading = strings.TrimSpace(heading)
				}

			case len(tds) == 2:
				var key string
				keyTexts := extractText(tds[0])
				valTexts := extractText(tds[1])

				if len(keyTexts) == 0 {
					key = "extras"
					if inner, ok := info[currentSection].(JsonObject); ok {
						if !currentSectionExtra {
							inner["extras"] = []any{}
							currentSectionExtra = true
						}
					}
					// break
				} else {
					key = keyTexts[0]

				}
				temp := strings.Join(valTexts, ", ")
				parts := strings.Split(temp, ",")
				var value any
				if len(parts) == 1 {
					value = strings.TrimSpace(parts[0])
				} else {
					var result []string
					for _, p := range parts {
						trimmed := strings.TrimSpace(p)
						if trimmed != "" {
							result = append(result, trimmed)
						}
					}
					value = result
				}
				if currentSection == "" {
					info[key] = value
				} else {
					if key != "" && key != "extras" {
						if inner, ok := info[currentSection].(JsonObject); ok {
							inner[key] = value
						}
					} else if key == "extras" {
						if inner, ok := info[currentSection].(JsonObject); ok {
							extras, _ := inner["extras"].([]any)
							inner["extras"] = append(extras, value)
						}
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Data == "tr" {
				for _, attr := range c.Attr {
					if attr.Key == "style" {
						continue
					}
				}
			}
			f(c)
		}
	}

	f(doc.FirstChild)

	result := JsonObject{
		"phone": body.PhoneName,
		"info":  info,
	}
	tempJSON, _ := json.MarshalIndent(result, "", "  ")
	return tempJSON, nil
}

func extractText(n *html.Node) []string {
	var texts []string
	var text func(*html.Node)
	text = func(node *html.Node) {
		if node.Type == html.TextNode {
			t := strings.TrimSpace(node.Data)
			if t != "" {
				texts = append(texts, t)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			text(c)
		}
	}
	text(n)
	return texts
}

func getTD(tr *html.Node) []*html.Node {
	var tds []*html.Node
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			tds = append(tds, c)
		}
	}
	return tds
}
