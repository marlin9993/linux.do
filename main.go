package main

import (
	"fmt"
	"github.com/playwright-community/playwright-go"
	"log"
	url_tools "net/url"
	"os"
	"strconv"
	"time"
)

var (
	url        = "https://linux.do/"
	email      = ""
	password   = ""
	topicCount = 3
)

func run() {
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start Playwright: %v", err)
	}
	defer pw.Stop()

	baseURL, _ := url_tools.Parse(url)
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	page.OnRequest(func(request playwright.Request) {
		log.Printf("Request: %s %s", request.Method(), request.URL())
	})

	page.OnResponse(func(response playwright.Response) {
		log.Printf("Response: %d %s", response.Status(), response.URL())
	})

	_, err = page.Goto(url)
	if err != nil {
		log.Fatalf("could not go to url: %v", err)
	}
	time.Sleep(5 * time.Second)
	// Login
	page.Click(".login-button .d-button-label")
	page.Fill("#login-account-name", email)
	page.Fill("#login-account-password", password)
	page.Click("#login-button")

	time.Sleep(5 * time.Second)

	userInfo, _ := page.QuerySelector("#toggle-current-user")
	if userInfo == nil {
		log.Fatalf("could not find user info")
	}
	ariaLabel, _ := userInfo.GetAttribute("aria-label")
	fmt.Println("======================================")
	fmt.Println("user_info:", ariaLabel)
	fmt.Println("======================================")

	// Get all links
	topicListBody, err := page.QuerySelector("tbody.topic-list-body")
	if err != nil {
		log.Fatalf("could not find topic list body: %v", err)
	}
	links, err := topicListBody.QuerySelectorAll("a.title.raw-link.raw-topic-link")
	if err != nil {
		log.Fatalf("could not find links: %v", err)
	}

	topics := make([]string, 0)
	for _, link := range links {
		href, _ := link.GetAttribute("href")
		relativeURL, _ := url_tools.Parse(href)
		absoluteURL := baseURL.ResolveReference(relativeURL)
		topics = append(topics, absoluteURL.String())
	}

	for i, topic := range topics {
		if i == topicCount {
			break
		}
		page.Goto(topic)
		time.Sleep(2 * time.Second)

		// Scroll down
		scrollIncrement := 100
		scrollDelay := 300 * time.Millisecond

		pageHeight, _ := page.Evaluate("() => document.body.scrollHeight")
		currentPosition := 0
		pageHeightInt := 0
		switch pageHeightT := pageHeight.(type) {
		case float64:
			pageHeightInt = int(pageHeightT)
		case float32:
			pageHeightInt = int(pageHeightT)
		case int:

			pageHeightInt = pageHeightT
		case int8:
			pageHeightInt = int(pageHeightT)
		case int16:
			pageHeightInt = int(pageHeightT)
		case int32:
			pageHeightInt = int(pageHeightT)
		case int64:
			pageHeightInt = int(pageHeightT)

		}

		for currentPosition < pageHeightInt {
			page.Evaluate(fmt.Sprintf("window.scrollBy(0, %d)", scrollIncrement))
			time.Sleep(scrollDelay)
			currentPosition += scrollIncrement
		}

		time.Sleep(2 * time.Second)
	}
}

func main() {
	email = os.Getenv("EMAIL")
	password = os.Getenv("PASSWORD")
	count := os.Getenv("TOPIC_COUNT")
	if count != "" {
		c, err := strconv.ParseInt(count, 10, 64)
		if err != nil {
			log.Fatalf("could not parse TOPIC_COUNT: %v", err)
		}
		topicCount = int(c)
	}
	if email == "" || password == "" {
		log.Fatalf("EMAIL and PASSWORD environment variables must be set")
	}
	run()
}
