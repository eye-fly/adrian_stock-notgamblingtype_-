package main

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/html"
)

type UniqueAvailability struct {
	Name          string
	CatalogNumber string
	Availability  string
	Link          string
}

func parseSearchResultsFromWeb(url string) ([]UniqueAvailability, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch search page: %w", err)
	}
	defer resp.Body.Close()

	// Use the html package to parse the response body from the request
	var results []UniqueAvailability
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return results, err
	}

	// var inTR, inH2, inStrong, inH3 bool
	// var current UniqueAvailability

	var processAllTRs func(*html.Node)
	processAllTRs = func(n *html.Node) {
		var stop = false
		if n.Type == html.ElementNode && n.Data == "tr" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.HasPrefix(attr.Val, "Availability") && !containsText(n, "Pokaż opcje") {
					processNode(n, &results)
					stop = true
				}
			}
		}
		if stop {
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			processAllTRs(c)
		}
	}

	processAllTRs(doc)

	// for _, r := range results {
	// 	println(r.Name)
	// }

	return results, nil
}
func containsText(n *html.Node, target string) bool {
	var found bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode && strings.Contains(strings.ToLower(n.Data), strings.ToLower(target)) {
			found = true
			return
		}
		for c := n.FirstChild; c != nil && !found; c = c.NextSibling {
			walk(c)
		}
	}

	walk(n)
	return found
}

func processNode(n *html.Node, results *[]UniqueAvailability) {
	var name, availability, katalog, link string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "h2":
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					name = strings.TrimSpace(n.FirstChild.Data)
				}
			case "h3":
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					availability = strings.TrimSpace(n.FirstChild.Data)
				}
			case "strong":
				if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					katalog = strings.TrimSpace(n.FirstChild.Data)
				}

			case "a":
				for _, attr := range n.Attr {
					if attr.Key == "href" && link == "" {
						link = "http://old.unique-meble.pl" + attr.Val
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(n)

	if name != "" || availability != "" {
		*results = append(*results, UniqueAvailability{
			Name:          name,
			Availability:  availability,
			CatalogNumber: katalog,
			Link:          link,
		})
		// fmt.Println("Name:         ", name)
		// fmt.Println("Availability: ", availability)
		// fmt.Println("Nr katalogowy:", katalog)
		// fmt.Println(strings.Repeat("-", 40))
	}
}

// ===================
func generateRestockReport(unavailable []Product, uniqueData []UniqueAvailability, outputCSV string) error {
	file, err := os.Create(outputCSV)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	writer.Write([]string{"Original Name", "Matched Unique Name", "Catalog Number", "URL", "Availability"})

	for _, prod := range unavailable {
		firstWord := getFirstWord(prod.Name)
		bestMatch := ""
		availability := "Not found"
		url := ""
		catalog := ""

		for _, u := range uniqueData {
			if strings.Contains(strings.ToLower(u.Name), firstWord) {
				bestMatch = u.Name
				availability = u.Availability
				catalog = u.CatalogNumber
				url = u.Link
				break // ✅ take the first good match
			}
		}

		writer.Write([]string{prod.Name, bestMatch, catalog, url, availability})
	}

	return nil
}

func getFirstWord(s string) string {
	words := tokenize(s)
	if len(words) > 0 {
		return words[0]
	}
	return ""
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, ".", "")
	return strings.Fields(s)
}
