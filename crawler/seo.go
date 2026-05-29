package crawler

import (
	"bytes"
	stdhtml "html"
	"strings"

	"golang.org/x/net/html"
)

func extractSEO(body []byte) SEOReport {
	document, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return SEOReport{}
	}

	seo := SEOReport{}

	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch strings.ToLower(node.Data) {
			case "title":
				if !seo.HasTitle {
					seo.HasTitle = true
					seo.Title = cleanSEOText(textContent(node))
				}
			case "meta":
				if !seo.HasDescription && isDescriptionMeta(node) {
					seo.HasDescription = true

					content, _ := attrValue(node, "content")
					seo.Description = cleanSEOText(content)
				}
			case "h1":
				if !seo.HasH1 {
					seo.HasH1 = true
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(document)

	return seo
}

func isDescriptionMeta(node *html.Node) bool {
	name, ok := attrValue(node, "name")
	if !ok {
		return false
	}

	return strings.EqualFold(cleanSEOText(name), "description")
}

func attrValue(node *html.Node, name string) (string, bool) {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val, true
		}
	}

	return "", false
}

func textContent(node *html.Node) string {
	parts := []string{}

	var walk func(current *html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			parts = append(parts, current.Data)
			return
		}

		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(node)

	return strings.Join(parts, " ")
}

func cleanSEOText(text string) string {
	return strings.Join(strings.Fields(stdhtml.UnescapeString(text)), " ")
}
