package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ibnaleem/gosearch/internal/models"
	"github.com/ibnaleem/gosearch/internal/search"
	"github.com/ibnaleem/gosearch/internal/theme"
)

const defaultStateFile = ".gosearch-watch-state.json"

// StateFile holds previous watch results keyed by username → site → status/url.
type StateFile struct {
	UpdatedAt time.Time                       `json:"updated_at"`
	Profiles  map[string]map[string]SiteState `json:"profiles"`
}

type SiteState struct {
	URL      string `json:"url"`
	Status   string `json:"status"`
	Category string `json:"category,omitempty"`
}

// LoadUsernames reads a newline-separated username list (comments with # allowed).
func LoadUsernames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, nil
}

func loadState(path string) StateFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return StateFile{Profiles: map[string]map[string]SiteState{}}
	}
	var s StateFile
	if err := json.Unmarshal(data, &s); err != nil || s.Profiles == nil {
		return StateFile{Profiles: map[string]map[string]SiteState{}}
	}
	return s
}

func saveState(path string, s StateFile) error {
	s.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Run re-checks usernames and prints only new or changed findings.
func Run(data models.Data, usernames []string, statePath string, opts search.Options) error {
	if statePath == "" {
		statePath = defaultStateFile
	}
	prev := loadState(statePath)
	next := StateFile{Profiles: map[string]map[string]SiteState{}}

	changes := 0
	for _, username := range usernames {
		theme.Yellowf("[*] Watching %s …", username).Println()
		opts.Username = username
		opts.Print = false
		results := search.Run(data, opts)

		siteMap := map[string]SiteState{}
		old := prev.Profiles[username]
		if old == nil {
			old = map[string]SiteState{}
		}

		for _, r := range results {
			key := r.Site + "\x00" + r.URL
			siteMap[key] = SiteState{URL: r.URL, Status: r.Status, Category: r.Category}
			prevState, seen := old[key]
			if !seen {
				if r.Status == "found" || r.Status == "uncertain" {
					theme.Greenf("[NEW] %s → %s (%s) %s", username, r.Site, r.Status, r.URL).Println()
					changes++
				}
				continue
			}
			if prevState.Status != r.Status {
				theme.Cyanf("[CHANGED] %s → %s: %s → %s (%s)", username, r.Site, prevState.Status, r.Status, r.URL).Println()
				changes++
			}
		}

		// Detect disappearances of previously found profiles.
		for key, st := range old {
			if _, ok := siteMap[key]; ok {
				continue
			}
			if st.Status == "found" || st.Status == "uncertain" {
				theme.Redf("[REMOVED] %s → %s was %s (%s)", username, strings.Split(key, "\x00")[0], st.Status, st.URL).Println()
				changes++
			}
		}

		next.Profiles[username] = siteMap
	}

	if err := saveState(statePath, next); err != nil {
		return err
	}
	fmt.Println()
	if changes == 0 {
		theme.Green("[*] No new or changed findings.").Println()
	} else {
		theme.Yellowf("[*] %d new/changed finding(s). State saved to %s", changes, statePath).Println()
	}
	return nil
}
