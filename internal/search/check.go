package search

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ibnaleem/gosearch/internal/models"
	"github.com/ibnaleem/gosearch/internal/theme"
)

// CheckSiteReport is the result of validating one site entry.
type CheckSiteReport struct {
	Site           string `json:"site"`
	Category       string `json:"category,omitempty"`
	OK             bool   `json:"ok"`
	ExistingUser   string `json:"existing_user"`
	ExistingStatus string `json:"existing_status"`
	MissingUser    string `json:"missing_user"`
	MissingStatus  string `json:"missing_status"`
	Detail         string `json:"detail,omitempty"`
}

func randomUsername() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 16)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "gosearchzz" + string(b)
}

// CheckSites probes each site with a known-existing username (if configured)
// and a random non-existing username, then reports broken matchers.
func CheckSites(data models.Data, opts Options) []CheckSiteReport {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 10
	}
	opts.Print = false

	var (
		mu       sync.Mutex
		reports  []CheckSiteReport
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

	theme.Yellowf("[*] Checking %d sites (known-existing + random missing)...", len(data.Websites)).Println()

	for _, website := range data.Websites {
		wg.Add(1)
		go func(website models.Website) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			acquireRate()

			existing := strings.TrimSpace(website.CheckUsername)
			if existing == "" {
				// Sensible defaults for well-known platforms.
				switch strings.ToLower(strings.TrimSpace(website.Name)) {
				case "github":
					existing = "torvalds"
				case "twitter/x", "twitter", "x":
					existing = "elonmusk"
				case "instagram":
					existing = "instagram"
				case "reddit":
					existing = "spez"
				case "tiktok":
					existing = "tiktok"
				case "twitch":
					existing = "ninja"
				case "youtube":
					existing = "Google"
				case "snapchat":
					existing = "teamsnapchat"
				case "bluesky":
					existing = "bsky.app"
				case "kick":
					existing = "xqc"
				case "threads":
					existing = "zuck"
				default:
					existing = "admin"
				}
			}
			missing := randomUsername()

			exOpts := opts
			exOpts.Username = existing
			missOpts := opts
			missOpts.Username = missing

			ex := probeWebsite(website, existing, exOpts)
			miss := probeWebsite(website, missing, missOpts)

			report := CheckSiteReport{
				Site:           website.Name,
				Category:       website.Category,
				ExistingUser:   existing,
				ExistingStatus: ex.Status,
				MissingUser:    missing,
				MissingStatus:  miss.Status,
			}

			// Healthy: existing → found (or uncertain for unknown), missing → not_found (or uncertain).
			broken := false
			detail := []string{}
			if website.ErrorType == "unknown" && len(website.Present) == 0 && len(website.Absent) == 0 && website.ResponseURL == "" {
				report.OK = true
				report.Detail = "unknown matcher (uncertain by design)"
			} else {
				if ex.Status == "not_found" {
					broken = true
					detail = append(detail, "known user reported not_found")
				}
				if miss.Status == "found" {
					broken = true
					detail = append(detail, "random user reported found (false positive)")
				}
				if ex.Status == "error" {
					detail = append(detail, "existing probe error: "+ex.Error)
				}
				if miss.Status == "error" {
					detail = append(detail, "missing probe error: "+miss.Error)
				}
				report.OK = !broken
				report.Detail = strings.Join(detail, "; ")
			}

			mu.Lock()
			reports = append(reports, report)
			mu.Unlock()

			if report.OK {
				theme.Greenf("[+] %s — ok (existing=%s/%s missing=%s/%s)", website.Name, existing, ex.Status, missing, miss.Status).Println()
			} else {
				theme.Redf("[-] %s — BROKEN (%s)", website.Name, report.Detail).Println()
			}
		}(website)
	}
	wg.Wait()

	brokenCount := 0
	for _, r := range reports {
		if !r.OK {
			brokenCount++
		}
	}
	fmt.Println()
	theme.Yellowf("[*] Checked %d sites, %d broken", len(reports), brokenCount).Println()
	return reports
}
