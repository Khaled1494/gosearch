package models

// Website describes how to probe a single site for a username.
// Existing errorType values remain supported; present/absent and
// response_url can be combined for stronger matching.
type Website struct {
	Name            string   `json:"name"`
	BaseURL         string   `json:"base_url"`
	URLProbe        string   `json:"url_probe,omitempty"`
	FollowRedirects bool     `json:"follow_redirects"`
	UserAgent       string   `json:"user_agent,omitempty"`
	ErrorType       string   `json:"errorType"`
	ErrorMsg        string   `json:"errorMsg,omitempty"`
	ErrorCode       int      `json:"errorCode,omitempty"`
	ResponseURL     string   `json:"response_url,omitempty"`
	Cookies         []Cookie `json:"cookies,omitempty"`
	Category        string   `json:"category,omitempty"`
	Present         []string `json:"present,omitempty"`
	Absent          []string `json:"absent,omitempty"`
	TimeoutMS       int      `json:"timeout_ms,omitempty"`
	Retries         int      `json:"retries,omitempty"`
	CheckUsername   string   `json:"check_username,omitempty"`
}

type Data struct {
	Websites []Website `json:"websites"`
}

type Cookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Result is one probe outcome for a username on a website.
type Result struct {
	Site     string `json:"site"`
	URL      string `json:"url"`
	Status   string `json:"status"` // found | not_found | uncertain | error
	Category string `json:"category,omitempty"`
	Error    string `json:"error,omitempty"`
}
