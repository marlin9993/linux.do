package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/playwright-community/playwright-go"
	"io"
	"log"
	"net/http"
	url_tools "net/url"
	"sync"
	"time"
)

var (
	url               = "https://linux.do/"
	defaultTopicCount = 3
)

func run(cookie string, topicCount int, logF logFunc, errF logFunc) error {
	pw, err := playwright.Run()
	if err != nil {
		log.Printf("could not start Playwright: %v", err)
		errF(fmt.Sprintf("could not start Playwright: %v", err))
		return errors.New("could not start Playwright")
	}
	defer pw.Stop()

	baseURL, _ := url_tools.Parse(url)
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		log.Printf("could not launch browser: %v", err)
		errF(fmt.Sprintf("could not launch browser: %v", err))
		return errors.New("could not launch browser")
	}
	defer browser.Close()

	context, err := browser.NewContext()
	if err != nil {
		log.Printf("could not create context: %v", err)
		errF(fmt.Sprintf("could not create context: %v", err))
		return errors.New("could not create context")
	}
	defer context.Close()
	context.AddCookies([]playwright.OptionalCookie{
		{
			Name:     "_t",
			Value:    cookie,
			URL:      nil,
			Domain:   playwright.String("linux.do"),
			Path:     playwright.String("/"),
			Expires:  nil,
			HttpOnly: playwright.Bool(true),
			Secure:   playwright.Bool(true),
			SameSite: playwright.SameSiteAttributeLax,
		},
	})

	page, err := context.NewPage()
	if err != nil {
		log.Printf("could not create page: %v", err)
		errF(fmt.Sprintf("could not create page: %v", err))
		return errors.New("could not create page")
	}

	page.OnRequest(func(request playwright.Request) {
		log.Printf("Request: %s %s", request.Method(), request.URL())
		logF(fmt.Sprintf("Request: %s %s", request.Method(), request.URL()))
	})

	page.OnResponse(func(response playwright.Response) {
		log.Printf("Response: %d %s", response.Status(), response.URL())
		logF(fmt.Sprintf("Response: %d %s", response.Status(), response.URL()))
	})

	_, err = page.Goto(url)
	if err != nil {
		log.Printf("could not go to url: %v", err)
		errF(fmt.Sprintf("could not go to url: %v", err))
		return errors.New("could not go to url")
	}

	time.Sleep(5 * time.Second)

	userInfo, _ := page.QuerySelector("#toggle-current-user")
	if userInfo == nil {
		log.Printf("could not find user info")
		errF("could not find user info")
		return errors.New("could not find user info")
	}
	ariaLabel, _ := userInfo.GetAttribute("aria-label")
	fmt.Println("======================================")
	fmt.Println("user_info:", ariaLabel)
	fmt.Println("======================================")

	// Get all links
	topicListBody, err := page.QuerySelector("tbody.topic-list-body")
	if err != nil {
		log.Printf("could not find topic list body: %v", err)
		errF("could not find topic list body")
		return errors.New("could not find topic list body")
	}
	links, err := topicListBody.QuerySelectorAll("a.title.raw-link.raw-topic-link")
	if err != nil {
		log.Printf("could not find links: %v", err)
		errF("could not find links")
		return errors.New("could not find links")
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
	return nil
}

var data struct {
	TopicCount int    `json:"topic_count"`
	Cookie     string `json:"cookie"`
}

type logFunc func(s string)

func toLogFunc(ctx context.Context, logc chan string) logFunc {
	return func(s string) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println(err)
			}
		}()
		select {
		case <-ctx.Done():
			return
		case logc <- s:
		}
	}

}

func main() {
	r := gin.Default()
	r.POST("/run", func(c *gin.Context) {

		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if data.TopicCount == 0 {
			data.TopicCount = defaultTopicCount
		}

		c.Header("Content-Type", "text/event-stream; charset=UTF-8")

		// 创建一个通道来发送事件
		logChan := make(chan string, 100)
		logChanOnce := sync.Once{}
		defer logChanOnce.Do(func() {
			close(logChan)
		})

		errChan := make(chan string, 1)
		errChanOnce := sync.Once{}
		defer errChanOnce.Do(func() {
			close(errChan)
		})

		go func() {
			// 关闭请求
			<-c.Request.Context().Done()
			fmt.Println("Client cancelled the request")
			logChanOnce.Do(func() {
				close(logChan)
			})
			errChanOnce.Do(func() {
				close(errChan)
			})

			for _ = range logChan {
				// do nothing
			}

		}()

		go func() {
			run(data.Cookie, data.TopicCount, toLogFunc(c.Request.Context(), logChan), toLogFunc(c.Request.Context(), errChan))
		}()

		c.Stream(func(w io.Writer) bool {
			select {
			case msg, ok := <-logChan:
				if !ok {
					return false
				}
				c.SSEvent("message", msg)
				return true
			case err, ok := <-errChan:
				if !ok {
					return false
				}
				c.SSEvent("error", err)
				return false
			}
		})
		return
	})
	r.Run(":8899")

}
