package search

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"

	"github.com/ibnaleem/gosearch/internal/config"
	"github.com/ibnaleem/gosearch/internal/matcher"
	"github.com/ibnaleem/gosearch/internal/models"
	github "github.com/ibnaleem/gosearch/internal/modules/github"
	"github.com/ibnaleem/gosearch/internal/modules/gravatar"
	"github.com/ibnaleem/gosearch/internal/modules/keybase"
	"github.com/ibnaleem/gosearch/internal/theme"
	"github.com/ibnaleem/gosearch/internal/utils"
)

// Options configures a Search run.
type Options struct {
	config.SearchOptions
	Username string
	Print    bool
	OnResult func(models.Result)
}

func newHTTPClient(followRedirects bool, timeout time.Duration) *http.Client {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: config.TLSConfig,
			Proxy:           http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Jar: nil,
	}
	if !followRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client
}

func buildRequest(website models.Website, url string) (*http.Request, error) {
	userAgent := config.DefaultUserAgent
	if website.UserAgent != "" {
		userAgent = website.UserAgent
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")

	for _, cookie := range website.Cookies {
		req.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}

	return req, nil
}

func decodeBody(res *http.Response) (io.ReadCloser, error) {
	switch res.Header.Get("Content-Encoding") {
	case "gzip":
		return gzip.NewReader(res.Body)
	case "deflate":
		return zlib.NewReader(res.Body)
	case "br":
		return io.NopCloser(brotli.NewReader(res.Body)), nil
	default:
		return res.Body, nil
	}
}

func siteTimeout(site models.Website, opts Options) time.Duration {
	if site.TimeoutMS > 0 {
		return time.Duration(site.TimeoutMS) * time.Millisecond
	}
	if opts.DefaultTimeout > 0 {
		return opts.DefaultTimeout
	}
	return 30 * time.Second
}

func siteRetries(site models.Website, opts Options) int {
	if site.Retries > 0 {
		return site.Retries
	}
	if opts.DefaultRetries > 0 {
		return opts.DefaultRetries
	}
	return 2
}

func probeWebsite(site models.Website, username string, opts Options) models.Result {
	profileURL := utils.BuildURL(site.BaseURL, username)
	result := models.Result{
		Site:     site.Name,
		URL:      profileURL,
		Category: site.Category,
		Status:   "error",
	}

	probeURL := profileURL
	if site.URLProbe != "" {
		probeURL = utils.BuildURL(site.URLProbe, username)
	}

	timeout := siteTimeout(site, opts)
	retries := siteRetries(site, opts)

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * 200 * time.Millisecond
			if backoff > 3*time.Second {
				backoff = 3 * time.Second
			}
			time.Sleep(backoff)
		}

		client := newHTTPClient(site.FollowRedirects, timeout)
		req, err := buildRequest(site, probeURL)
		if err != nil {
			lastErr = err
			continue
		}

		res, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		reader, err := decodeBody(res)
		if err != nil {
			res.Body.Close()
			lastErr = err
			continue
		}
		bodyBytes, err := io.ReadAll(io.LimitReader(reader, 4<<20))
		res.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		finalURL := ""
		if res.Request != nil && res.Request.URL != nil {
			finalURL = res.Request.URL.String()
		}

		verdict := matcher.Evaluate(site, matcher.ProbeInput{
			StatusCode: res.StatusCode,
			Body:       string(bodyBytes),
			FinalURL:   finalURL,
		}, username)

		result.Status = verdict.String()
		result.Error = ""
		return result
	}

	if lastErr != nil {
		result.Error = lastErr.Error()
	} else {
		result.Error = "probe failed"
	}
	return result
}

func emit(opts Options, r models.Result) {
	if opts.OnResult != nil {
		opts.OnResult(r)
	}
	if !opts.Print {
		return
	}
	switch r.Status {
	case "found":
		theme.Green("[+] ", r.Site, ":", r.URL).Println()
		utils.WriteToFile(opts.Username, r.URL+"\n")
		config.Count.Add(1)
	case "uncertain":
		if !opts.NoFalsePositives {
			theme.Yellowf("[?] %s: %s", r.Site, r.URL).Println()
			utils.WriteToFile(opts.Username, "[?] "+r.URL+"\n")
			config.Count.Add(1)
		}
	}
}

// Run searches all websites for username using bounded concurrency,
// optional global rate limiting, per-site timeouts, and retries.
func Run(data models.Data, opts Options) []models.Result {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 20
	}

	var (
		mu       sync.Mutex
		results  []models.Result
		wg       sync.WaitGroup
		sem      = make(chan struct{}, opts.Concurrency)
		rateMu   sync.Mutex
		lastReq  time.Time
		rateWait = opts.RateLimit
	)

	acquireRate := func() {
		if rateWait <= 0 {
			return
		}
		rateMu.Lock()
		defer rateMu.Unlock()
		if !lastReq.IsZero() {
			if wait := rateWait - time.Since(lastReq); wait > 0 {
				time.Sleep(wait)
			}
		}
		lastReq = time.Now()
	}

	for _, website := range data.Websites {
		wg.Add(1)
		go func(website models.Website) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			acquireRate()

			name := strings.TrimSpace(website.Name)
			switch name {
			case "GitHub":
				if opts.Print {
					githubUser, err := github.UnmarshalGitHubUser(opts.Username)
					if err == nil {
						fmt.Println()
						github.DisplayGitHubInfo(githubUser, opts.Username)
					}
				}
				r := probeWebsite(website, opts.Username, opts)
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				emit(opts, r)
				return
			case "Gravatar":
				if opts.Print {
					fmt.Println()
					gravatar.SearchGravatar(opts.Username)
				}
				r := models.Result{Site: name, URL: utils.BuildURL(website.BaseURL, opts.Username), Category: website.Category, Status: "uncertain"}
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				emit(opts, r)
				return
			case "Keybase":
				if opts.Print {
					keybaseUser, err := keybase.UnmarshalKeybaseUser(opts.Username)
					if err == nil {
						fmt.Println()
						keybase.DisplayKeybaseInfo(keybaseUser, opts.Username)
					}
				}
				r := probeWebsite(website, opts.Username, opts)
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
				emit(opts, r)
				return
			}

			r := probeWebsite(website, opts.Username, opts)
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
			emit(opts, r)
		}(website)
	}
	wg.Wait()
	return results
}

// Search keeps the historical WaitGroup-based API used by main.
func Search(data models.Data, username string, noFalsePositives bool, wg *sync.WaitGroup) {
	defer func() {
		// Historical callers Add(len(websites)) and expect each site to Done once.
		// Run manages its own WaitGroup, so we Done all here.
		for range data.Websites {
			wg.Done()
		}
	}()

	opts := Options{
		SearchOptions: config.DefaultSearchOptions(),
		Username:      username,
		Print:         true,
	}
	opts.NoFalsePositives = noFalsePositives
	Run(data, opts)
}

// SearchWithOptions is used by CLI modes that need structured results.
func SearchWithOptions(ctx context.Context, data models.Data, opts Options) []models.Result {
	_ = ctx
	return Run(data, opts)
}
