// Created by github.com/ibnaleem & contributors
// Contribute & support: github.com/ibnaleem/gosearch

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inancgumus/screen"
	"github.com/joho/godotenv"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"

	"github.com/ibnaleem/gosearch/internal/config"
	"github.com/ibnaleem/gosearch/internal/modules/breachdirectory"
	github "github.com/ibnaleem/gosearch/internal/modules/github"
	"github.com/ibnaleem/gosearch/internal/modules/hudsonrock"
	"github.com/ibnaleem/gosearch/internal/modules/proxynova"
	"github.com/ibnaleem/gosearch/internal/output"
	"github.com/ibnaleem/gosearch/internal/search"
	"github.com/ibnaleem/gosearch/internal/serve"
	"github.com/ibnaleem/gosearch/internal/theme"
	"github.com/ibnaleem/gosearch/internal/utils"
	"github.com/ibnaleem/gosearch/internal/watch"
)

func usage() {
	fmt.Println(`Usage:
  gosearch -u <username> [flags]
  gosearch serve [--port 8080]
  gosearch --check-sites
  gosearch --watch <usernames.txt>

Flags:
  -u, --username string        Username to search
  -o string                    Directory for output files (default: .)
  --output csv|json|html       Also write a structured report
  --no-false-positives         Hide uncertain (yellow) results
  -b, --breach-directory key   BreachDirectory API key
  --concurrency int            Max parallel site probes (default 20)
  --rate-limit duration        Min delay between request starts (e.g. 50ms)
  --timeout duration           Default per-site timeout (default 30s)
  --retries int                Retries with exponential backoff (default 2)
  --check-sites                Validate each site entry (known + random user)
  --watch file                 Re-check usernames listed in file; print diffs
  --watch-state file           Watch state JSON (default .gosearch-watch-state.json)

Issues: https://github.com/ibnaleem/gosearch/issues`)
}

func main() {
	godotenv.Load() // silently ignored if no .env file

	if len(os.Args) > 1 && os.Args[1] == "serve" {
		runServe(os.Args[2:])
		return
	}

	usernameFlag := flag.String("u", "", "Username to search")
	usernameFlagLong := flag.String("username", "", "Username to search")
	noFalsePositivesFlag := flag.Bool("no-false-positives", false, "Do not show false positives")
	breachDirectoryAPIKey := flag.String("b", "", "Search Breach Directory with an API Key")
	breachDirectoryAPIKeyLong := flag.String("breach-directory", "", "Search Breach Directory with an API Key")
	outdirFlag := flag.String("o", "", "Directory to save the output files (default: current directory)")
	outputFormatFlag := flag.String("output", "", "Report format: csv, json, or html (also accepts a directory path for backward compatibility)")
	concurrencyFlag := flag.Int("concurrency", 20, "Max parallel site probes")
	rateLimitFlag := flag.Duration("rate-limit", 0, "Minimum delay between request starts")
	timeoutFlag := flag.Duration("timeout", 30*time.Second, "Default per-site HTTP timeout")
	retriesFlag := flag.Int("retries", 2, "Retries with exponential backoff")
	checkSitesFlag := flag.Bool("check-sites", false, "Test each site with known-existing and random usernames")
	watchFlag := flag.String("watch", "", "File listing usernames to re-check; print only new/changed findings")
	watchStateFlag := flag.String("watch-state", ".gosearch-watch-state.json", "JSON file storing previous watch results")

	flag.Usage = usage
	flag.Parse()

	outdir, format := resolveOutput(*outdirFlag, *outputFormatFlag)
	if outdir != "" {
		config.OutputDir = outdir
	}

	data, err := utils.UnmarshalJSON()
	if err != nil {
		fmt.Printf("Error unmarshalling json: %v\n", err)
		os.Exit(1)
	}

	searchOpts := search.Options{
		SearchOptions: config.SearchOptions{
			Concurrency:      *concurrencyFlag,
			RateLimit:        *rateLimitFlag,
			DefaultTimeout:   *timeoutFlag,
			DefaultRetries:   *retriesFlag,
			NoFalsePositives: *noFalsePositivesFlag,
		},
		Print: true,
	}

	if *checkSitesFlag {
		reports := search.CheckSites(data, searchOpts)
		if format != "" {
			// Reuse JSON writer shape via a light marshal.
			path := fmt.Sprintf("check-sites_%s.json", time.Now().Format("2006-01-02_150405"))
			if config.OutputDir != "" && config.OutputDir != "." {
				path = config.OutputDir + "/" + path
			}
			b, _ := json.MarshalIndent(reports, "", "  ")
			_ = os.WriteFile(path, b, 0o644)
			fmt.Println("Wrote", path)
		}
		return
	}

	if *watchFlag != "" {
		usernames, err := watch.LoadUsernames(*watchFlag)
		if err != nil {
			fmt.Printf("Error reading watch file: %v\n", err)
			os.Exit(1)
		}
		if len(usernames) == 0 {
			fmt.Println("Watch file contains no usernames")
			os.Exit(1)
		}
		searchOpts.Print = false
		if err := watch.Run(data, usernames, *watchStateFlag, searchOpts); err != nil {
			fmt.Printf("Watch error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	username := *usernameFlag
	if username == "" {
		username = *usernameFlagLong
	}
	if username == "" {
		usage()
		os.Exit(1)
	}

	var wg sync.WaitGroup

	screen.Clear()
	fmt.Print(config.ASCII)
	fmt.Println(config.VERSION)
	fmt.Println(strings.Repeat("⎯", 85))
	fmt.Println(":: Username                              : ", username)
	fmt.Println(":: Websites                              : ", len(data.Websites))
	fmt.Println(":: Concurrency                           : ", *concurrencyFlag)
	if *rateLimitFlag > 0 {
		fmt.Println(":: Rate limit                            : ", *rateLimitFlag)
	}
	if *noFalsePositivesFlag {
		fmt.Println(":: No False Positives                    : ", *noFalsePositivesFlag)
	}
	fmt.Println(strings.Repeat("⎯", 85))
	fmt.Println()

	if !*noFalsePositivesFlag {
		fmt.Println("[!] A yellow link indicates that I was unable to verify whether the username exists on the platform.")
	}

	start := time.Now()

	searchOpts.Username = username
	searchOpts.Print = true
	results := search.Run(data, searchOpts)

	fmt.Println()
	fmt.Println()

	github.DisplayEmailsFromCommits(username)

	fmt.Println()
	fmt.Println()

	github.DisplayGPGKeys(username)

	fmt.Println()
	fmt.Println()

	github.DisplaySSHKeys(username)

	fmt.Println()
	fmt.Println()

	github.DisplaySocialAccounts(username)

	fmt.Println()
	fmt.Println()

	followers, err := github.FetchFollowers(username)
	if err != nil {
		theme.Redf("[-] Error fetching followers: %v", err).Println()
	} else {
		mutualFollowers, err := github.FindMutualFollowers(followers, username)
		if err != nil {
			theme.Redf("[-] Error finding mutual followers: %v", err).Println()
		} else {
			github.DisplayMutualFollowers(mutualFollowers)
		}
	}

	fmt.Println()
	fmt.Println()

	wg.Add(1)
	utils.WriteToFile(username, strings.Repeat("⎯", 85))
	theme.Yellow("[*] Searching HudsonRock's Cybercrime Intelligence Database...").Println()
	go hudsonrock.HudsonRock(username, &wg)
	wg.Wait()

	apikey := *breachDirectoryAPIKey
	if apikey == "" {
		apikey = *breachDirectoryAPIKeyLong
	}
	if apikey != "" {
		fmt.Println()
		fmt.Println()

		wg.Add(1)
		go breachdirectory.SearchBreachDirectory(username, apikey, &wg)
		wg.Wait()
	}

	fmt.Println()
	fmt.Println()

	wg.Add(1)
	utils.WriteToFile(username, strings.Repeat("⎯", 85))
	go proxynova.SearchProxyNova(username, &wg)
	wg.Wait()

	fmt.Println()
	fmt.Println()

	domains := utils.BuildDomains(username)
	wg.Add(1)
	go search.SearchDomains(username, domains, &wg)
	wg.Wait()

	fmt.Println()
	fmt.Println()

	elapsed := time.Since(start)

	table := tablewriter.NewTable(os.Stdout, tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{Borders: tw.BorderNone})))
	table.Append(theme.Bold("Number of profiles found"), theme.Red(config.Count.Load()))
	table.Append(theme.Bold("Total time taken"), theme.Green(elapsed))
	if err := table.Render(); err != nil {
		log.Printf("table render failed: %v", err)
	}

	utils.WriteToFile(username, ":: Number of profiles found              : "+strconv.Itoa(int(config.Count.Load())))
	utils.WriteToFile(username, ":: Total time taken                      : "+elapsed.String())

	if format != "" {
		path, err := output.Write(format, config.OutputDir, username, results)
		if err != nil {
			fmt.Printf("Error writing %s report: %v\n", format, err)
		} else {
			theme.Greenf("[+] Wrote %s report: %s", format, path).Println()
		}
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8080, "HTTP port")
	concurrency := fs.Int("concurrency", 20, "Max parallel site probes")
	rateLimit := fs.Duration("rate-limit", 0, "Minimum delay between request starts")
	timeout := fs.Duration("timeout", 30*time.Second, "Default per-site HTTP timeout")
	retries := fs.Int("retries", 2, "Retries with exponential backoff")
	_ = fs.Parse(args)

	data, err := utils.UnmarshalJSON()
	if err != nil {
		fmt.Printf("Error loading data.json: %v\n", err)
		os.Exit(1)
	}

	srv := &serve.Server{
		Addr: fmt.Sprintf(":%d", *port),
		Data: data,
		Options: search.Options{
			SearchOptions: config.SearchOptions{
				Concurrency:    *concurrency,
				RateLimit:      *rateLimit,
				DefaultTimeout: *timeout,
				DefaultRetries: *retries,
			},
		},
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// resolveOutput keeps -o as the output directory. --output accepts csv|json|html;
// if a legacy path is passed to --output, treat it as a directory.
func resolveOutput(outdir, outputFlag string) (dir, format string) {
	dir = outdir
	v := strings.TrimSpace(outputFlag)
	if v == "" {
		return dir, ""
	}
	switch strings.ToLower(v) {
	case "csv", "json", "html":
		return dir, strings.ToLower(v)
	default:
		// Backward compatible: --output <directory>
		if dir == "" {
			dir = v
		}
		return dir, ""
	}
}
