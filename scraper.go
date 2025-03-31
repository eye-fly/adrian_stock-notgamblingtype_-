package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

const baseURL = "https://fotelewarszawa.pl"

type Product struct {
	Name      string
	URL       string
	Available bool
}

func main() {
	baseListURL := "https://fotelewarszawa.pl/42-fotele-biurowe?resultsPerPage=100&q=Marka-Unique"
	allProducts := []Product{}
	page := 1
	totalChecked := 0

	fmt.Println("🔍 Starting product scrape...")

	for {
		url := fmt.Sprintf("%s&page=%d", baseListURL, page)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("❌ Failed to fetch page %d: %v", page, err)
			break
		}
		defer resp.Body.Close()

		pageProducts := extractProductsFromSearch(resp)
		if len(pageProducts) == 0 {
			fmt.Printf("✅ No products found on page %d. Done scraping.\n", page)
			break
		}

		allProducts = append(allProducts, pageProducts...)
		fmt.Printf("📦 Page %d: found %d products (total: %d)\n", page, len(pageProducts), len(allProducts))
		page++

		// break
	}

	fmt.Printf("🛠️ Checking availability for %d products...\n", len(allProducts))

	allProducts[0].URL = "https://fotelewarszawa.pl/fotele-biurowe/909-patron-fotel-biurowy-.html"
	allProducts[0].Name = "Ares Fotel gabinetowy skóra naturalna 2 kolory"
	allProducts = allProducts[:10]
	for i := range allProducts {
		allProducts[i].Available = checkAvailability(allProducts[i].URL)
		totalChecked++
		fmt.Printf("✅ [%d/%d] Checked: %s\n", totalChecked, len(allProducts), allProducts[i].Name)
	}

	print(allProducts[0].Name, allProducts[0].URL)
	// Step 1: Filter out unavailable products
	var unavailable []Product
	for _, p := range allProducts {
		if !p.Available {
			unavailable = append(unavailable, p)
		}
	}

	fmt.Printf("🛒 Found %d unavailable products.\n", len(unavailable))

	// Step 2: Parse Unique availability file
	uniqueData, err := parseSearchResultsFromWeb("http://old.unique-meble.pl/dostepnosc.html")
	if err != nil {
		log.Fatalf("Failed to fetch unique availability data: %v", err)
	}

	// Step 3: Match and write report
	err = generateRestockReport(unavailable, uniqueData, "restock_report.csv")
	if err != nil {
		log.Fatalf("Failed to write restock report: %v", err)
	}
	fmt.Println("✅ Restock report written to restock_report.csv")
}

func extractProductsFromSearch(resp *http.Response) []Product {
	var products []Product

	doc, err := html.Parse(resp.Body)
	if err != nil {
		fmt.Println("Error parsing HTML:", err)
		return products
	}

	var findSearchWrapper func(*html.Node)
	findSearchWrapper = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, attr := range n.Attr {
				if attr.Key == "id" && attr.Val == "content-wrapper" {
					// ✅ Instead of stopping here, go deeper
					findProducts(n, &products)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findSearchWrapper(c)
		}
	}

	findSearchWrapper(doc)
	return products
}
func findProducts(n *html.Node, products *[]Product) {
	if n.Type == html.ElementNode && n.Data == "article" {
		for _, attr := range n.Attr {
			if attr.Key == "class" && strings.Contains(attr.Val, "product-miniature") {
				product := extractProductFromArticle(n)
				if product.Name != "" && product.URL != "" {
					*products = append(*products, product)
				}
			}
		}
	}
	// 🔁 Keep scanning deeper
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		findProducts(c, products)
	}
}
func extractProductFromArticle(n *html.Node) Product {
	var name, url string

	var inProductTitle bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "h2" {
				for _, attr := range n.Attr {
					if attr.Key == "class" && strings.Contains(attr.Val, "product-title") {
						inProductTitle = true
					}
				}
			}

			if inProductTitle && n.Data == "a" {
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						url = attr.Val
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						name = strings.TrimSpace(c.Data)
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}

		// reset when leaving h2
		if n.Type == html.ElementNode && n.Data == "h2" {
			inProductTitle = false
		}
	}

	walk(n)

	return Product{
		Name: name,
		URL:  url,
	}
}

func checkAvailability(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error checking product page %s: %v", url, err)
		return false
	}
	defer resp.Body.Close()

	z := html.NewTokenizer(resp.Body)

	depth := 0
	inProductQuantity := false
	inAddDiv := false

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}

		t := z.Token()

		switch t.Type {
		case html.StartTagToken:
			if t.Data == "div" {
				for _, attr := range t.Attr {
					if attr.Key == "class" && attr.Val == "product-quantity" {
						inProductQuantity = true
						depth = 1
					} else if inProductQuantity && attr.Key == "class" && attr.Val == "add" {
						inAddDiv = true
					}
				}
				if inProductQuantity {
					depth++
				}
			}

			if inAddDiv && t.Data == "button" {
				return true
			}

		case html.EndTagToken:
			if inProductQuantity && t.Data == "div" {
				depth--
				if depth == 0 {
					inProductQuantity = false
					inAddDiv = false
				}
			}
		}
	}

	return false
}
