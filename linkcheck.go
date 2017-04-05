// Modifications 2017 by Ad Hoc. Original copyright/license below.
//
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The linkcheck command finds missing links in the given website.
// It crawls a URL recursively and notes URLs and URL fragments
// that it's seen and prints a report of missing links at the end.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

var (
	root     = flag.String("root", "http://localhost:8000", "Root to crawl")
	verbose  = flag.Bool("verbose", false, "verbose")
	crawlers = flag.Int("crawlers", runtime.NumCPU(), "number of concurrent crawlers")
)

var base *url.URL // the parsed root, used to resolve references

var excludePaths []string

var wg sync.WaitGroup        // outstanding fetches
var urlq = make(chan string) // URLs to crawl

// urlFrag is a URL and its optional #fragment (without the #)
type urlFrag struct {
	url, frag string
}

var (
	mu          sync.Mutex
	crawled     = make(map[string]bool)      // URL without fragment -> true
	neededFrags = make(map[urlFrag][]string) // URL#frag -> who needs it
)

// Owned by crawlLoop goroutines:
var (
	linkSources   = make(map[string][]string) // url no fragment -> sources
	linkSourcesMu sync.Mutex
	fragExists    = make(map[urlFrag]bool)
	fragExistsMu  sync.Mutex
	problems      []string
)

func isAnchor(n *html.Node) bool {
	return n.Type == html.ElementNode && n.Data == "a"
}

func href(n *html.Node) string {
	for _, attr := range n.Attr {
		if attr.Key == "href" {
			return attr.Val
		}
	}
	return ""
}

var invalidProtos = []string{
	"mailto:",
	"javascript:",
	"tel:",
}

func excludeLink(ref string) bool {
	for _, proto := range invalidProtos {
		if strings.HasPrefix(ref, proto) {
			return true
		}
	}
	for _, prefix := range excludePaths {
		if strings.HasPrefix(ref, prefix) {
			return true
		}
	}
	return false
}

// parses URL and resolves references
func parseUrl(ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		panic(err)
	}
	return base.ResolveReference(u).String()
}

func getLinks(body string) (links []string) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		log.Printf("ERROR: parsing HTML: %v", err)
		return
	}

	// TODO(paulsmith): global seen map
	seen := map[string]bool{}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if isAnchor(n) {
			ref := href(n)
			ref = parseUrl(ref)
			if !seen[ref] {
				seen[ref] = true
				links = append(links, ref)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return
}

var idRx = regexp.MustCompile(`\bid=['"]?([^\s'">]+)`)

func pageIDs(body string) (ids []string) {
	mv := idRx.FindAllStringSubmatch(body, -1)
	for _, m := range mv {
		ids = append(ids, m[1])
	}
	return
}

// url may contain a #fragment, and the fragment is then noted as needing to exist.
func crawl(url string, sourceURL string) {
	mu.Lock()
	defer mu.Unlock()
	var frag string
	if i := strings.Index(url, "#"); i >= 0 {
		frag = url[i+1:]
		url = url[:i]
		if frag != "" {
			uf := urlFrag{url, frag}
			neededFrags[uf] = append(neededFrags[uf], sourceURL)
		}
	}
	if crawled[url] {
		return
	}
	crawled[url] = true

	wg.Add(1)
	go func() {
		urlq <- url
	}()
}

func addProblem(url, errmsg string) {
	msg := fmt.Sprintf("Error on %s: %s (from %s)", url, errmsg, linkSources[url])
	if *verbose {
		log.Print(msg)
	}
	problems = append(problems, msg)
}

func crawlLoop() {
	for url := range urlq {
		if err := doCrawl(url); err != nil {
			addProblem(url, err.Error())
		}
	}
}

func doCrawl(url string) error {
	defer wg.Done()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	// Handle redirects.
	if res.StatusCode/100 == 3 {
		newURL, err := res.Location()
		if err != nil {
			return fmt.Errorf("resolving redirect: %v", err)
		}
		if !strings.HasPrefix(newURL.String(), *root) {
			// Skip off-site redirects.
			return nil
		}
		crawl(newURL.String(), url)
		return nil
	}
	if res.StatusCode != 200 {
		return errors.New(res.Status)
	}
	// Don't recurse through external links -- just check them once
	if strings.HasPrefix(url, *root) {

		buf := bufio.NewReader(res.Body)
		// http.DetectContentType only uses first 512 bytes
		peek, err := buf.Peek(512)
		if err != nil {
			log.Fatalf("Error initially reading %s body: %v", url, err)
		}

		if ct := http.DetectContentType(peek); !strings.HasPrefix(ct, "text/html") {
			if *verbose {
				log.Printf("Skipping %s, content-type %s", url, ct)
			}
			return nil
		}

		slurp, err := ioutil.ReadAll(buf)
		if err != nil {
			log.Fatalf("Error reading %s body: %v", url, err)
		}
		if *verbose {
			log.Printf("Len of %s: %d", url, len(slurp))
		}
		body := string(slurp)
		for _, ref := range getLinks(body) {
			if *verbose {
				log.Printf("  links to %s", ref)
			}
			if excludeLink(ref) {
				if *verbose {
					log.Printf("    excluding %s", ref)
				}
				continue
			}
			dest := ref
			linkSourcesMu.Lock()
			linkSources[dest] = append(linkSources[dest], url)
			linkSourcesMu.Unlock()
			crawl(dest, url)
		}
		for _, id := range pageIDs(body) {
			if *verbose {
				log.Printf(" url %s has #%s", url, id)
			}
			fragExistsMu.Lock()
			fragExists[urlFrag{url, id}] = true
			fragExistsMu.Unlock()
		}
	}
	return nil
}

func main() {
	flag.Parse()

	var err error
	base, err = url.Parse(*root)
	if err != nil {
		log.Fatalf("parsing root URL: %v", err)
	}
	if base.Path == "" {
		base.Path = "/"
	}

	if *crawlers < 1 {
		log.Fatalf("need at least one crawler")
	}

	if *verbose {
		log.Printf("starting %d crawlers", *crawlers)
	}

	for i := 0; i < *crawlers; i++ {
		go crawlLoop()
	}

	crawl(base.String(), "")

	wg.Wait()
	close(urlq)
	for uf, needers := range neededFrags {
		if !fragExists[uf] {
			problems = append(problems, fmt.Sprintf("Missing fragment for %+v from %v", uf, needers))
		}
	}

	for _, s := range problems {
		fmt.Println(s)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
}
