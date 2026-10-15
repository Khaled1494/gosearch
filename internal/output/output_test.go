package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ibnaleem/gosearch/internal/models"
)

func TestWriteFormats(t *testing.T) {
	dir := t.TempDir()
	results := []models.Result{
		{Site: "Example", URL: "https://example.com/bob", Status: "found", Category: "social"},
		{Site: "Other", URL: "https://other.test/bob", Status: "not_found", Category: "gulf"},
	}
	for _, format := range []string{"csv", "json", "html"} {
		path, err := Write(format, dir, "bob", results)
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if len(data) == 0 {
			t.Fatalf("%s empty", format)
		}
		if format == "html" && !strings.Contains(string(data), "example.com/bob") {
			t.Fatalf("html missing link")
		}
		if format == "csv" && !strings.Contains(string(data), "Example") {
			t.Fatalf("csv missing site")
		}
	}
	if _, err := Write("xml", dir, "bob", results); err == nil {
		t.Fatal("expected error for unsupported format")
	}
	_ = filepath.Base(dir)
}

func TestSanitize(t *testing.T) {
	if sanitize("../bob!") != "___bob_" {
		t.Fatalf("unexpected sanitize: %q", sanitize("../bob!"))
	}
}
