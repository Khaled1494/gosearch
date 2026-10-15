package matcher

import (
	"testing"

	"github.com/ibnaleem/gosearch/internal/models"
)

func TestEvaluateStatusCode(t *testing.T) {
	site := models.Website{ErrorType: "status_code"}
	if got := Evaluate(site, ProbeInput{StatusCode: 200}, "user"); got != Found {
		t.Fatalf("200 => %s, want found", got)
	}
	if got := Evaluate(site, ProbeInput{StatusCode: 404}, "user"); got != NotFound {
		t.Fatalf("404 => %s, want not_found", got)
	}
}

func TestEvaluateErrorMsgAbsent(t *testing.T) {
	site := models.Website{ErrorType: "errorMsg", ErrorMsg: "User not found"}
	if got := Evaluate(site, ProbeInput{StatusCode: 200, Body: "hello User not found"}, "user"); got != NotFound {
		t.Fatalf("absent hit => %s, want not_found", got)
	}
	if got := Evaluate(site, ProbeInput{StatusCode: 200, Body: "profile ok"}, "user"); got != Found {
		t.Fatalf("absent miss => %s, want found", got)
	}
}

func TestEvaluateProfilePresence(t *testing.T) {
	site := models.Website{ErrorType: "profilePresence", ErrorMsg: `"username":"bob"`}
	if got := Evaluate(site, ProbeInput{StatusCode: 200, Body: `{"username":"bob"}`}, "bob"); got != Found {
		t.Fatalf("present hit => %s, want found", got)
	}
	if got := Evaluate(site, ProbeInput{StatusCode: 200, Body: `{}`}, "bob"); got != NotFound {
		t.Fatalf("present miss => %s, want not_found", got)
	}
}

func TestEvaluatePresentAbsentArrays(t *testing.T) {
	site := models.Website{
		ErrorType: "status_code",
		Present:   []string{"display_name"},
		Absent:    []string{"account_not_found"},
	}
	if got := Evaluate(site, ProbeInput{StatusCode: 200, Body: "display_name"}, "u"); got != Found {
		t.Fatalf("present+status => %s, want found", got)
	}
	if got := Evaluate(site, ProbeInput{StatusCode: 200, Body: "account_not_found"}, "u"); got != Uncertain {
		t.Fatalf("conflict => %s, want uncertain", got)
	}
	if got := Evaluate(site, ProbeInput{StatusCode: 200, Body: "account_not_found display_name"}, "u"); got != Uncertain {
		t.Fatalf("both present and absent => %s, want uncertain", got)
	}
}

func TestEvaluateResponseURL(t *testing.T) {
	site := models.Website{
		ErrorType:   "response_url",
		ResponseURL: "https://example.com/search?q={}",
	}
	if got := Evaluate(site, ProbeInput{StatusCode: 200, FinalURL: "https://example.com/u/bob"}, "bob"); got != Found {
		t.Fatalf("other redirect => %s, want found", got)
	}
	if got := Evaluate(site, ProbeInput{StatusCode: 200, FinalURL: "https://example.com/search?q=bob"}, "bob"); got != NotFound {
		t.Fatalf("not-found redirect => %s, want not_found", got)
	}
}

func TestEvaluateUnknown(t *testing.T) {
	site := models.Website{ErrorType: "unknown"}
	if got := Evaluate(site, ProbeInput{StatusCode: 200, Body: "x"}, "u"); got != Uncertain {
		t.Fatalf("unknown => %s, want uncertain", got)
	}
}

func TestVerdictString(t *testing.T) {
	if Found.String() != "found" || NotFound.String() != "not_found" || Uncertain.String() != "uncertain" {
		t.Fatal("unexpected Verdict.String values")
	}
}
