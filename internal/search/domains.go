package search

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"

	"github.com/ibnaleem/gosearch/internal/config"
	"github.com/ibnaleem/gosearch/internal/theme"
	"github.com/ibnaleem/gosearch/internal/utils"
)

// SearchDomains checks common TLDs for the username as a domain name.
func SearchDomains(username string, domains []string, wg *sync.WaitGroup) {
	defer wg.Done()

	client := &http.Client{Timeout: 15 * time.Second}
	theme.Yellow("[*] Searching ", len(domains), " domains with the username ", username, "...").Println()

	domainCount := 0
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("NO", "DOMAIN", "STATUS")

	x := 0
	for _, domain := range domains {
		url := "http://" + domain

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			fmt.Printf("Error creating request for %s: %v\n", domain, err)
			continue
		}
		req.Header.Set("User-Agent", config.DefaultUserAgent)
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

		resp, err := client.Do(req)
		if err != nil {
			var netErr net.Error
			ok := errors.As(err, &netErr)
			noSuchHostError := strings.Contains(err.Error(), "no such host")
			networkTimeoutError := ok && netErr.Timeout()

			if !noSuchHostError && !networkTimeoutError {
				fmt.Printf("Error sending request for %s: %v\n", domain, err)
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			x++
			table.Append(x, domain, theme.Green(http.StatusOK))
			utils.WriteToFile(username, "[+] 200 OK: "+domain)
			domainCount++
		}
		resp.Body.Close()
	}

	if err := table.Render(); err != nil {
		log.Printf("table render failed: %v", err)
	}
	if domainCount > 0 {
		theme.Greenf("[+] Found %d domains with the username %s", domainCount, username).Println()
		utils.WriteToFile(username, "[+] Found "+strconv.Itoa(domainCount)+" domains with the username: "+username)
	} else {
		theme.Redf("[-] No domains found with the username %s", username).Println()
		utils.WriteToFile(username, "[-] No domains found with the username: "+username)
	}
}
