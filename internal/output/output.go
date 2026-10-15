package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ibnaleem/gosearch/internal/models"
)

// Write writes results in the requested format (csv|json|html) under dir.
// Returns the path written.
func Write(format, dir, username string, results []models.Result) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stamp := time.Now().Format("2006-01-02_150405")
	base := fmt.Sprintf("%s_%s", sanitize(username), stamp)

	switch format {
	case "csv":
		path := filepath.Join(dir, base+".csv")
		return path, writeCSV(path, results)
	case "json":
		path := filepath.Join(dir, base+".json")
		return path, writeJSON(path, username, results)
	case "html":
		path := filepath.Join(dir, base+".html")
		return path, writeHTML(path, username, results)
	default:
		return "", fmt.Errorf("unsupported output format %q (use csv, json, or html)", format)
	}
}

func sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
	if s == "" {
		return "report"
	}
	return s
}

func writeCSV(path string, results []models.Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"site", "url", "status", "category", "error"})
	for _, r := range results {
		_ = w.Write([]string{r.Site, r.URL, r.Status, r.Category, r.Error})
	}
	w.Flush()
	return w.Error()
}

func writeJSON(path, username string, results []models.Result) error {
	payload := map[string]interface{}{
		"username": username,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"counts":       countByStatus(results),
		"results":      results,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func countByStatus(results []models.Result) map[string]int {
	counts := map[string]int{"found": 0, "not_found": 0, "uncertain": 0, "error": 0}
	for _, r := range results {
		counts[r.Status]++
	}
	return counts
}

func writeHTML(path, username string, results []models.Result) error {
	counts := countByStatus(results)
	var rows strings.Builder
	for _, r := range results {
		rows.WriteString("<tr>")
		rows.WriteString("<td>" + html.EscapeString(r.Site) + "</td>")
		rows.WriteString(`<td><a href="` + html.EscapeString(r.URL) + `" target="_blank" rel="noopener">` + html.EscapeString(r.URL) + "</a></td>")
		rows.WriteString(`<td class="status ` + html.EscapeString(r.Status) + `">` + html.EscapeString(r.Status) + "</td>")
		rows.WriteString("<td>" + html.EscapeString(r.Category) + "</td>")
		rows.WriteString("</tr>\n")
	}

	doc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>GoSearch report — %s</title>
<style>
  :root { --bg:#0f1419; --panel:#1a2332; --text:#e7ecf3; --muted:#9aa7b8; --accent:#3d9cf0; --found:#3ecf8e; --not:#e85d5d; --unc:#e6b84d; --err:#9aa7b8; }
  * { box-sizing: border-box; }
  body { margin:0; font-family: "Segoe UI", "Noto Sans", sans-serif; background: linear-gradient(160deg,#0f1419 0%%,#1a2838 100%%); color:var(--text); min-height:100vh; }
  main { max-width:1100px; margin:0 auto; padding:2rem 1.25rem 3rem; }
  h1 { font-size:1.75rem; margin:0 0 .25rem; letter-spacing:-0.02em; }
  .sub { color:var(--muted); margin-bottom:1.5rem; }
  .summary { display:flex; flex-wrap:wrap; gap:.75rem; margin-bottom:1.5rem; }
  .chip { background:var(--panel); padding:.5rem .9rem; border-radius:6px; font-size:.9rem; }
  .chip b { color:var(--accent); }
  table { width:100%%; border-collapse:collapse; background:var(--panel); border-radius:8px; overflow:hidden; }
  th, td { padding:.65rem .8rem; text-align:left; border-bottom:1px solid #2a3548; font-size:.92rem; }
  th { cursor:pointer; user-select:none; color:var(--muted); font-weight:600; background:#152033; }
  th:hover { color:var(--text); }
  a { color:var(--accent); text-decoration:none; word-break:break-all; }
  a:hover { text-decoration:underline; }
  .status.found { color:var(--found); font-weight:600; }
  .status.not_found { color:var(--not); }
  .status.uncertain { color:var(--unc); }
  .status.error { color:var(--err); }
  .filters { margin-bottom:1rem; }
  .filters label { margin-right:1rem; color:var(--muted); font-size:.9rem; }
</style>
</head>
<body>
<main>
  <h1>GoSearch — %s</h1>
  <p class="sub">Generated %s · click column headers to sort</p>
  <div class="summary">
    <div class="chip">Found <b>%d</b></div>
    <div class="chip">Not found <b>%d</b></div>
    <div class="chip">Uncertain <b>%d</b></div>
    <div class="chip">Errors <b>%d</b></div>
    <div class="chip">Total <b>%d</b></div>
  </div>
  <div class="filters">
    <label><input type="checkbox" id="onlyFound"> Found only</label>
  </div>
  <table id="report">
    <thead>
      <tr>
        <th data-col="0">Site</th>
        <th data-col="1">URL</th>
        <th data-col="2">Status</th>
        <th data-col="3">Category</th>
      </tr>
    </thead>
    <tbody>
%s
    </tbody>
  </table>
</main>
<script>
(function(){
  var table = document.getElementById('report');
  var tbody = table.tBodies[0];
  var sortDir = {};
  table.querySelectorAll('th').forEach(function(th){
    th.addEventListener('click', function(){
      var col = +th.getAttribute('data-col');
      sortDir[col] = !sortDir[col];
      var rows = Array.prototype.slice.call(tbody.rows);
      rows.sort(function(a,b){
        var av = a.cells[col].textContent.trim().toLowerCase();
        var bv = b.cells[col].textContent.trim().toLowerCase();
        if (av < bv) return sortDir[col] ? -1 : 1;
        if (av > bv) return sortDir[col] ? 1 : -1;
        return 0;
      });
      rows.forEach(function(r){ tbody.appendChild(r); });
    });
  });
  document.getElementById('onlyFound').addEventListener('change', function(e){
    Array.prototype.forEach.call(tbody.rows, function(row){
      var status = row.cells[2].textContent.trim();
      row.style.display = (!e.target.checked || status === 'found') ? '' : 'none';
    });
  });
})();
</script>
</body>
</html>`,
		html.EscapeString(username),
		html.EscapeString(username),
		html.EscapeString(time.Now().Format(time.RFC1123)),
		counts["found"], counts["not_found"], counts["uncertain"], counts["error"], len(results),
		rows.String(),
	)
	return os.WriteFile(path, []byte(doc), 0o644)
}
