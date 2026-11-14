package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/mxschmitt/playwright-go"
)

const (
	checkInterval    = 5 * time.Minute
	renderWait       = 5 * time.Second
	maxDiscordLength = 1980
)

type config struct {
	TargetLocations   []string `json:"target_locations"`
	RFDSources        []string `json:"RFD_sources"`
	HotDealsURL       string   `json:"RFD_HOT_DEAL_URL"`
	DealLinkPrefix    string   `json:"RFD_DEAL_LINK_PREFIX"`
	DiscordWebhookURL string   `json:"DISCORD_WEBHOOK_URL"`
}

type deal struct {
	Title  string
	Link   string
	Source string
}

type gpuInfo struct {
	Name string
	URL  string
}

type discordPayload struct {
	Content  string `json:"content"`
	Username string `json:"username"`
}

var (
	dealIDPattern = regexp.MustCompile(`-(\d+)/`)
	httpClient    = &http.Client{Timeout: 20 * time.Second}
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	configPath, err := findConfigPath()
	if err != nil {
		return err
	}
	settings, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	dealsPath := filepath.Join(filepath.Dir(configPath), "pushed_deals.txt")
	pushedDeals, err := readDealIDs(dealsPath)
	if err != nil {
		return fmt.Errorf("read pushed deals: %w", err)
	}
	keepLatestDealIDs(pushedDeals, 200)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	defer func() {
		if err := saveDealIDs(dealsPath, pushedDeals); err != nil {
			log.Printf("save pushed deals: %v", err)
		}
	}()

	for {
		log.Printf("=== Starting check at %s ===", time.Now().Format("2006-01-02 15:04:05"))
		if err := checkRFD(ctx, settings, pushedDeals); err != nil {
			log.Printf("RFD check failed: %v", err)
		}

		timer := time.NewTimer(checkInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Printf("Shutting down; saving pushed deals")
			return nil
		case <-timer.C:
		}
	}
}

func findConfigPath() (string, error) {
	candidates := []string{
		filepath.Join("conf", "config.json"),
		filepath.Join("..", "conf", "config.json"),
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("could not find conf/config.json (run from the repository root or the go directory)")
}

func loadConfig(path string) (config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var settings config
	if err := json.Unmarshal(data, &settings); err != nil {
		return config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	if settings.HotDealsURL == "" {
		return config{}, fmt.Errorf("config %q is missing RFD_HOT_DEAL_URL", path)
	}
	if settings.DiscordWebhookURL == "" {
		return config{}, fmt.Errorf("config %q is missing DISCORD_WEBHOOK_URL", path)
	}
	return settings, nil
}

func loadPage(url string) (string, error) {
	pw, err := playwright.Run()
	if err != nil {
		return "", fmt.Errorf("start Playwright: %w", err)
	}
	defer func() {
		if err := pw.Stop(); err != nil {
			log.Printf("stop Playwright: %v", err)
		}
	}()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--no-sandbox",
		},
	})
	if err != nil {
		return "", fmt.Errorf("launch Chromium: %w", err)
	}
	defer func() {
		if err := browser.Close(); err != nil {
			log.Printf("close Chromium: %v", err)
		}
	}()

	browserContext, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0 Safari/537.36"),
		Viewport:  &playwright.Size{Width: 1280, Height: 900},
	})
	if err != nil {
		return "", fmt.Errorf("create browser context: %w", err)
	}

	page, err := browserContext.NewPage()
	if err != nil {
		return "", fmt.Errorf("create page: %w", err)
	}
	if _, err := page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(120000),
	}); err != nil {
		return "", fmt.Errorf("navigate to %q: %w", url, err)
	}
	page.WaitForTimeout(float64(renderWait / time.Millisecond))
	html, err := page.Content()
	if err != nil {
		return "", fmt.Errorf("read rendered page: %w", err)
	}
	return html, nil
}

func checkRFD(ctx context.Context, settings config, pushedDeals map[int64]struct{}) error {
	html, err := loadPage(settings.HotDealsURL)
	if err != nil {
		return err
	}

	newDeals, err := parseRFDDeals(html, settings, pushedDeals)
	if err != nil {
		return err
	}
	if len(newDeals) == 0 {
		return nil
	}
	return sendRFDNotifications(ctx, settings.DiscordWebhookURL, newDeals)
}

func parseRFDDeals(html string, settings config, pushedDeals map[int64]struct{}) ([]deal, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse RFD page: %w", err)
	}

	var newDeals []deal
	doc.Find("li.topic-card.topic").Each(func(_ int, listing *goquery.Selection) {
		link := listing.Find("a.topic-card-info").First()
		relativeURL, exists := link.Attr("href")
		if !exists {
			return
		}

		title := strings.TrimSpace(link.Find("h3.thread_title").First().Text())
		if title == "" {
			title = "Title not found"
		}
		source := strings.TrimSpace(link.Find("div.dealer_name").First().Text())
		if source == "" {
			source = "Source not found"
		}

		matchesSource := false
		for _, candidate := range settings.RFDSources {
			if strings.Contains(source, candidate) {
				matchesSource = true
				break
			}
		}
		if !matchesSource {
			return
		}

		fullURL := settings.DealLinkPrefix + relativeURL
		id, ok := getDealID(fullURL)
		if !ok {
			log.Printf("skip deal with unrecognized ID: %s", fullURL)
			return
		}
		if _, exists := pushedDeals[id]; exists {
			log.Printf("passed: [%s] %s -- %d -- %d", source, title, id, len(pushedDeals))
			return
		}

		pushedDeals[id] = struct{}{}
		newDeals = append(newDeals, deal{Title: title, Link: fullURL, Source: source})
		log.Printf("new deal: [%s] %s -- %d -- %d", source, title, id, len(pushedDeals))
	})
	return newDeals, nil
}

func getDealID(link string) (int64, bool) {
	matches := dealIDPattern.FindAllStringSubmatch(link, -1)
	if len(matches) == 0 {
		return 0, false
	}
	id, err := strconv.ParseInt(matches[len(matches)-1][1], 10, 64)
	return id, err == nil
}

func sendRFDNotifications(ctx context.Context, webhookURL string, deals []deal) error {
	message := "🚨 **RFD alart_new** 🚨\n"
	var messages []string
	for _, item := range deals {
		dealText := "[" + item.Source + "] " + item.Title + "\n" + item.Link + "\n"
		if utf8.RuneCountInString(message)+utf8.RuneCountInString(dealText) < maxDiscordLength {
			message += dealText
		} else {
			messages = append(messages, message)
			message = dealText
		}
	}
	messages = append(messages, message)

	var failures []error
	for _, content := range messages {
		if err := postDiscordNotification(ctx, webhookURL, discordPayload{
			Content:  content,
			Username: "RFD Bot",
		}); err != nil {
			failures = append(failures, fmt.Errorf("send RFD Discord notification: %w", err))
		}
		select {
		case <-ctx.Done():
			return errors.Join(append(failures, ctx.Err())...)
		case <-time.After(5 * time.Second):
		}
	}
	return errors.Join(failures...)
}

func checkGPUStock(ctx context.Context, settings config, gpu gpuInfo) error {
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, gpu.URL, nil)
	if err != nil {
		return fmt.Errorf("create GPU page request: %w", err)
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("fetch GPU page: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("fetch GPU page: HTTP %s", response.Status)
	}

	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return fmt.Errorf("parse GPU page: %w", err)
	}
	onlineText := strings.TrimSpace(doc.Find("#onlineinfo > div > div > p.mt-1.text-dark.f-16.fm-xs-SF-Pro-Display-Medium").First().Text())
	onlineStatus := "Sold out"
	if onlineText != "" && !strings.Contains(onlineText, "Sold out online") {
		onlineStatus = "In Stock"
	}

	targets := make(map[string]struct{}, len(settings.TargetLocations))
	for _, location := range settings.TargetLocations {
		targets[location] = struct{}{}
	}
	storeStatus := make(map[string]string)
	doc.Find("#collapseON > div").First().Children().Each(func(_ int, row *goquery.Selection) {
		location := strings.TrimSpace(row.Find("span.col-3.fm-SegoeUI-Bold.fm-xs-SF-Pro-Display-Bold").First().Text())
		if _, wanted := targets[location]; !wanted {
			return
		}
		count := strings.TrimSpace(row.Find("span.bg-E3E9F8").First().Text())
		if count == "" {
			count = "0"
		}
		storeStatus[location] = count
	})

	log.Printf("GPU Name: %s", gpu.Name)
	log.Printf("Online: %s", onlineStatus)
	hasStoreStock := false
	for _, location := range settings.TargetLocations {
		count := storeStatus[location]
		if count == "" {
			count = "0"
		}
		log.Printf("%s: %s", location, count)
		stock, err := strconv.Atoi(count)
		if err == nil && stock > 0 {
			hasStoreStock = true
		}
	}
	if onlineStatus == "In Stock" || hasStoreStock {
		return sendGPUNotification(ctx, settings.DiscordWebhookURL, gpu.Name, onlineStatus, settings.TargetLocations, storeStatus)
	}
	return nil
}

func sendGPUNotification(ctx context.Context, webhookURL, name, onlineStatus string, locations []string, storeStatus map[string]string) error {
	var message strings.Builder
	message.WriteString("🚨 **GPU IN STOCK ALERT** 🚨\n\n")
	message.WriteString("**" + name + "**\n")
	message.WriteString("**Online:** " + onlineStatus + "\n")
	if len(storeStatus) > 0 {
		message.WriteString("**Store Availability:**\n")
		for _, location := range locations {
			if count, ok := storeStatus[location]; ok {
				message.WriteString(location + ": " + count + "\n")
			}
		}
	}
	message.WriteString("\n🕒 " + time.Now().Format("2006-01-02 15:04:05"))
	return postDiscordNotification(ctx, webhookURL, discordPayload{
		Content:  message.String(),
		Username: "GPU Stock Bot",
	})
}

func postDiscordNotification(ctx context.Context, webhookURL string, payload discordPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Discord payload: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Discord request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("post to Discord: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("Discord returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func readDealIDs(path string) (map[int64]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	deals := make(map[int64]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		id, err := strconv.ParseInt(strings.TrimSpace(scanner.Text()), 10, 64)
		if err == nil {
			deals[id] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read pushed deal IDs: %w", err)
	}
	return deals, nil
}

func saveDealIDs(path string, deals map[int64]struct{}) error {
	ids := make([]int64, 0, len(deals))
	for id := range deals {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })

	var contents strings.Builder
	for _, id := range ids {
		fmt.Fprintf(&contents, "%d\n", id)
	}
	if err := os.WriteFile(path, []byte(contents.String()), 0644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

func keepLatestDealIDs(deals map[int64]struct{}, limit int) {
	if len(deals) <= limit {
		return
	}
	ids := make([]int64, 0, len(deals))
	for id := range deals {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	for _, id := range ids[limit:] {
		delete(deals, id)
	}
}
