package internals

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

const phoneName = "iPhone 17"
const baseURLPDB = "https://phonedb.net"

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
	// collyCollector.OnHTML("div.container", func(container *colly.HTMLElement) {
	// 	if container.DOM.Find("div.content_block").Length() == 0 {
	// 		return
	// 	}

	// 	container.DOM.Find("div.content_block").Each(func(_ int, block *goquery.Selection) {

	// 		if found {
	// 			return
	// 		}

	// 		titleDiv := block.Find("div.content_block_title")
	// 		if titleDiv.Length() == 0 {
	// 			return
	// 		}

	// 		titleText := strings.TrimSpace(titleDiv.Find("a").First().Text())

	// 		if !strings.Contains(strings.ToLower(titleText), strings.ToLower(phone)) {
	// 			return
	// 		}

	// 		var detailHref string
	// 		block.Find("a").Each(func(_ int, a *goquery.Selection) {
	// 			if strings.EqualFold(strings.TrimSpace(a.Text()), "all details") {
	// 				if href, exists := a.Attr("href"); exists {
	// 					detailHref = href
	// 				}
	// 			}
	// 		})

	// 		if detailHref == "" {
	// 			return
	// 		}

	// 		match = PhoneResult{Title: titleText, DetailHref: detailHref}
	// 		found = true
	// 	})
	// })

	collyCollector.OnRequest(func(r *colly.Request) {
		fmt.Printf("→ Search POST: %s\n", r.URL)
	})
	collyCollector.OnResponse(func(r *colly.Response) {
		fmt.Printf("← Response: %d (%d bytes\n", r.StatusCode, len(r.Body))
		err := os.WriteFile("./htm.txt", r.Body, 0644)
		if err != nil {
			panic(fmt.Errorf("could not write file %s: %w", err))
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

func FetchDetailTable(result PhoneResult) (string, error) {
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
