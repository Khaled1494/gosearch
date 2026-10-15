package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ibnaleem/gosearch/internal/config"
	"github.com/ibnaleem/gosearch/internal/models"
	"github.com/ibnaleem/gosearch/internal/output"
	"github.com/ibnaleem/gosearch/internal/search"
)

// Server hosts a minimal username search UI with SSE streaming.
type Server struct {
	Addr    string
	Data    models.Data
	Options search.Options
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/search", s.handleSearchSSE)
	mux.HandleFunc("/api/download", s.handleDownload)
	fmt.Printf("GoSearch UI listening on http://%s\n", s.Addr)
	return http.ListenAndServe(s.Addr, mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleSearchSSE(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	opts := s.Options
	opts.Username = username
	opts.Print = false

	var (
		mu      sync.Mutex
		results []models.Result
	)

	opts.OnResult = func(res models.Result) {
		mu.Lock()
		results = append(results, res)
		mu.Unlock()
		payload, _ := json.Marshal(res)
		fmt.Fprintf(w, "event: result\ndata: %s\n\n", payload)
		flusher.Flush()
	}

	start := time.Now()
	fmt.Fprintf(w, "event: start\ndata: {\"username\":%q,\"sites\":%d}\n\n", username, len(s.Data.Websites))
	flusher.Flush()

	search.Run(s.Data, opts)

	mu.Lock()
	summary := map[string]interface{}{
		"elapsed_ms": time.Since(start).Milliseconds(),
		"total":      len(results),
		"found":      countStatus(results, "found"),
	}
	mu.Unlock()
	payload, _ := json.Marshal(summary)
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", payload)
	flusher.Flush()

	// Stash last results in memory for download (per-username key via query on download).
	storeResults(username, results)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		format = "html"
	}
	results := loadResults(username)
	if len(results) == 0 {
		http.Error(w, "no results for username; run a search first", http.StatusNotFound)
		return
	}
	dir := config.OutputDir
	if dir == "" {
		dir = "."
	}
	path, err := output.Write(format, dir, username, results)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, path)
}

func countStatus(results []models.Result, status string) int {
	n := 0
	for _, r := range results {
		if r.Status == status {
			n++
		}
	}
	return n
}

var (
	resultMu sync.Mutex
	lastRuns = map[string][]models.Result{}
)

func storeResults(username string, results []models.Result) {
	resultMu.Lock()
	defer resultMu.Unlock()
	cp := make([]models.Result, len(results))
	copy(cp, results)
	lastRuns[username] = cp
}

func loadResults(username string) []models.Result {
	resultMu.Lock()
	defer resultMu.Unlock()
	return lastRuns[username]
}

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>GoSearch</title>
<style>
  :root { --bg:#101820; --card:#1b2838; --text:#eef3f8; --muted:#94a3b8; --accent:#4da3ff; --ok:#34d399; }
  * { box-sizing:border-box; }
  body { margin:0; min-height:100vh; font-family:"Segoe UI","Noto Sans Arabic","Noto Sans",sans-serif;
    background: radial-gradient(1200px 600px at 10% -10%, #1e3a5f 0%, transparent 55%),
                linear-gradient(165deg,#0c1219,#152033 60%,#101820); color:var(--text); }
  main { max-width:900px; margin:0 auto; padding:2.5rem 1.25rem; }
  h1 { margin:0 0 .35rem; font-size:2rem; letter-spacing:-0.03em; }
  p.lead { color:var(--muted); margin:0 0 1.5rem; }
  form { display:flex; gap:.6rem; flex-wrap:wrap; margin-bottom:1.25rem; }
  input[type=text] { flex:1; min-width:200px; padding:.75rem 1rem; border-radius:8px; border:1px solid #2d3f55;
    background:#0f1722; color:var(--text); font-size:1rem; }
  button { padding:.75rem 1.1rem; border:0; border-radius:8px; background:var(--accent); color:#041018;
    font-weight:700; cursor:pointer; }
  button:disabled { opacity:.6; cursor:wait; }
  .actions a { color:var(--accent); margin-right:1rem; font-size:.95rem; }
  #status { color:var(--muted); margin:.5rem 0 1rem; min-height:1.25rem; }
  table { width:100%; border-collapse:collapse; background:var(--card); border-radius:10px; overflow:hidden; }
  th,td { padding:.55rem .7rem; border-bottom:1px solid #243447; text-align:left; font-size:.9rem; }
  th { color:var(--muted); background:#152033; }
  a { color:var(--accent); word-break:break-all; }
  .found { color:var(--ok); font-weight:600; }
</style>
</head>
<body>
<main>
  <h1>GoSearch</h1>
  <p class="lead">Enter a username to stream live results across 300+ sites.</p>
  <form id="form">
    <input type="text" id="username" name="username" placeholder="username" required autocomplete="off">
    <button type="submit" id="go">Search</button>
  </form>
  <div class="actions" id="downloads" hidden>
    <a id="dl-html" href="#">Download HTML</a>
    <a id="dl-json" href="#">Download JSON</a>
    <a id="dl-csv" href="#">Download CSV</a>
  </div>
  <div id="status"></div>
  <table>
    <thead><tr><th>Site</th><th>URL</th><th>Status</th><th>Category</th></tr></thead>
    <tbody id="rows"></tbody>
  </table>
</main>
<script>
(function(){
  var form = document.getElementById('form');
  var rows = document.getElementById('rows');
  var status = document.getElementById('status');
  var go = document.getElementById('go');
  var downloads = document.getElementById('downloads');
  var es = null;

  form.addEventListener('submit', function(e){
    e.preventDefault();
    var u = document.getElementById('username').value.trim();
    if (!u) return;
    if (es) es.close();
    rows.innerHTML = '';
    downloads.hidden = true;
    go.disabled = true;
    status.textContent = 'Starting…';
    var found = 0, total = 0;
    es = new EventSource('/api/search?username=' + encodeURIComponent(u));
    es.addEventListener('start', function(ev){
      var d = JSON.parse(ev.data);
      status.textContent = 'Searching ' + d.sites + ' sites for ' + d.username + '…';
    });
    es.addEventListener('result', function(ev){
      var r = JSON.parse(ev.data);
      total++;
      if (r.status === 'found') found++;
      if (r.status !== 'found' && r.status !== 'uncertain') return;
      var tr = document.createElement('tr');
      tr.innerHTML = '<td>' + esc(r.site) + '</td>' +
        '<td><a href="' + esc(r.url) + '" target="_blank" rel="noopener">' + esc(r.url) + '</a></td>' +
        '<td class="' + (r.status==='found'?'found':'') + '">' + esc(r.status) + '</td>' +
        '<td>' + esc(r.category||'') + '</td>';
      rows.appendChild(tr);
      status.textContent = 'Live: ' + found + ' found / ' + total + ' checked';
    });
    es.addEventListener('done', function(ev){
      var d = JSON.parse(ev.data);
      status.textContent = 'Done in ' + d.elapsed_ms + 'ms — ' + d.found + ' found / ' + d.total + ' sites';
      go.disabled = false;
      downloads.hidden = false;
      document.getElementById('dl-html').href = '/api/download?username=' + encodeURIComponent(u) + '&format=html';
      document.getElementById('dl-json').href = '/api/download?username=' + encodeURIComponent(u) + '&format=json';
      document.getElementById('dl-csv').href = '/api/download?username=' + encodeURIComponent(u) + '&format=csv';
      es.close();
    });
    es.onerror = function(){
      status.textContent = 'Connection closed or failed.';
      go.disabled = false;
      if (es) es.close();
    };
  });

  function esc(s){
    return String(s||'').replace(/[&<>"']/g, function(c){
      return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c];
    });
  }
})();
</script>
</body>
</html>
`
