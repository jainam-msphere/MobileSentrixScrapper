package internals

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/valyala/fasthttp"
	"golang.org/x/net/html"
)

type JsonObject map[string]any

type BodyObj struct {
	HtmlString  string `json:"html"`
	PhoneName   string `json:"phone"`
	CompanyName string `json:"company"`
}

func GSMParser(ctx *fasthttp.RequestCtx) {
	var body struct {
		HtmlString  string `json:"html"`
		PhoneName   string `json:"phone"`
		CompanyName string `json:"company"`
	}

	if err := json.Unmarshal(ctx.PostBody(), &body); err != nil {
		log.Fatal("some issue occured while unmarshaling")
		return
	}

	doc, _ := html.Parse(strings.NewReader(body.HtmlString))
	var pointer *html.Node
	var multipleData []string
	var temp = JsonObject{}
	var extractJson func(*html.Node)
	var category, topic string
	occurance := 0
	extractJson = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "th" || n.Data == "td" {
				pointer = n
			}
			if n.Data == "tr" {
				occurance = 0
			}
		}
		if n.Type == html.TextNode && pointer != nil && n.NextSibling != nil && n.NextSibling.Data == "hr" {
			multipleData = append(multipleData, n.Data)
		} else if n.Type == html.TextNode && pointer != nil && n.PrevSibling != nil && n.PrevSibling.Data == "hr" {
			multipleData = append(multipleData, n.Data)
		} else if n.Type == html.TextNode && pointer != nil {
			cleanText := strings.TrimSpace(n.Data)
			if cleanText != "" {
				if pointer.Data == "th" {
					category = cleanText
					if _, exists := temp[category]; !exists {
						temp[category] = make(JsonObject)
					}
				} else if pointer.Data == "td" && category != "" {
					occurance++
					switch occurance {
					case 1:
						topic = cleanText
					case 2:
						if inner, ok := temp[category].(JsonObject); ok {
							inner[topic] = cleanText
						}
						occurance = 0
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractJson(c)
		}
		if n == pointer {
			if len(multipleData) > 0 {
				if inner, ok := temp[category].(JsonObject); ok {
					inner["OtherInfo"] = multipleData
					multipleData = []string{}
				}
			}
			pointer = nil
		}
	}
	extractJson(doc.FirstChild)
	tempJSON, _ := json.MarshalIndent(temp, "", "  ")
	saveFile(tempJSON, body.PhoneName, body.CompanyName)
	ctx.SetStatusCode(fasthttp.StatusOK)
}

func PDBParser(body BodyObj) ([]byte, error) {
	// var body struct {
	// 	HtmlString  string `json:"html"`
	// 	PhoneName   string `json:"phone"`
	// 	CompanyName string `json:"company"`
	// }

	// if err := json.Unmarshal(ctx.PostBody(), &body); err != nil {
	// 	log.Fatal("some issue occured while unmarshaling")
	// 	return
	// }

	doc, err := html.Parse(strings.NewReader(body.HtmlString))
	if err != nil {
		// ctx.Response.SetStatusCode(400)
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
				// case len(tds) >2:

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
	saveFile(tempJSON, body.PhoneName, body.CompanyName)
	return tempJSON, nil
}

func saveFile(data []byte, fileName string, folderName string) {
	path := filepath.Join(".", "database", folderName)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(fmt.Errorf("could not create folder %s: %w", folderName, err))
	}
	fullPath := filepath.Join(path, fileName+".json")
	err = os.WriteFile(fullPath, data, 0644)
	if err != nil {
		panic(fmt.Errorf("could not write file %s: %w", fileName, err))
	}
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
