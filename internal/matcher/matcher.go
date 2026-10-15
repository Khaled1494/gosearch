package matcher

import (
	"strings"

	"github.com/ibnaleem/gosearch/internal/models"
)

// Verdict is the outcome of evaluating a probe response.
type Verdict int

const (
	Found Verdict = iota
	NotFound
	Uncertain
)

func (v Verdict) String() string {
	switch v {
	case Found:
		return "found"
	case NotFound:
		return "not_found"
	default:
		return "uncertain"
	}
}

// ProbeInput is the HTTP evidence used to decide existence.
type ProbeInput struct {
	StatusCode int
	Body       string
	FinalURL   string
}

// Evaluate decides whether a username exists from response evidence.
// It supports the legacy errorType modes and optional present/absent
// body strings plus final redirect URL checks.
func Evaluate(site models.Website, in ProbeInput, username string) Verdict {
	signals := 0
	foundVotes := 0
	notFoundVotes := 0

	vote := func(ok bool) {
		signals++
		if ok {
			foundVotes++
		} else {
			notFoundVotes++
		}
	}

	errorType := strings.TrimSpace(site.ErrorType)

	// HTTP status: treat >=400 as not found unless a custom ErrorCode is set.
	useStatus := errorType == "status_code" || site.ErrorCode != 0
	if useStatus {
		if site.ErrorCode != 0 {
			if in.StatusCode >= 400 && in.StatusCode == site.ErrorCode {
				vote(false)
			} else if in.StatusCode >= 400 {
				vote(false)
			} else {
				vote(in.StatusCode != site.ErrorCode)
			}
		} else {
			vote(in.StatusCode > 0 && in.StatusCode < 400)
		}
	} else if in.StatusCode >= 400 && errorType != "unknown" {
		// Hard failure for non-unknown types without an explicit status check.
		vote(false)
	}

	// Absent strings: presence in body means username does not exist.
	absent := append([]string{}, site.Absent...)
	if errorType == "errorMsg" && site.ErrorMsg != "" {
		absent = append(absent, site.ErrorMsg)
	}
	for _, s := range absent {
		if s == "" {
			continue
		}
		vote(!strings.Contains(in.Body, s))
	}

	// Present strings: must appear in body for an existing profile.
	present := append([]string{}, site.Present...)
	if errorType == "profilePresence" && site.ErrorMsg != "" {
		present = append(present, site.ErrorMsg)
	}
	for _, s := range present {
		if s == "" {
			continue
		}
		vote(strings.Contains(in.Body, s))
	}

	// Final redirect URL: ResponseURL is the "not found" destination.
	if site.ResponseURL != "" {
		notFoundURL := strings.Replace(site.ResponseURL, "{}", username, 1)
		if in.FinalURL == "" {
			vote(false)
		} else {
			vote(in.FinalURL != notFoundURL)
		}
	}

	if signals == 0 {
		if errorType == "unknown" || errorType == "" {
			return Uncertain
		}
		return Uncertain
	}

	if notFoundVotes > 0 && foundVotes == 0 {
		return NotFound
	}
	if foundVotes > 0 && notFoundVotes == 0 {
		return Found
	}
	// Conflicting signals → uncertain rather than a false positive.
	return Uncertain
}
