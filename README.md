<p align='center'>
<img src='img/gosearch-logo.png' height=50% width=50%><br>
<i>This project heavily relies on contributors, please see <a href="#contributing">Contributing</a> for more details.</i><br>
<code>go install github.com/ibnaleem/gosearch@latest</code>
</p>

<p align="center">
  <img src="https://github.com/ibnaleem/gosearch/actions/workflows/go.yml/badge.svg?event=push" alt="GitHub Actions Badge"> <img src="https://img.shields.io/github/last-commit/ibnaleem/gosearch"> <img src="https://img.shields.io/github/commit-activity/w/ibnaleem/gosearch"> <img src="https://img.shields.io/github/contributors/ibnaleem/gosearch"> <img alt="Number of websites" src="https://img.shields.io/badge/websites-300+-blue"> <img alt="GitHub repo size" src="https://img.shields.io/github/repo-size/ibnaleem/gosearch"> <img alt="GitHub License" src="https://img.shields.io/github/license/ibnaleem/gosearch">
</p>

> [!WARNING]  
> For your own privacy and security, ***never download GoSearch from anywhere other than this repository.***

---

## English

### Overview

GoSearch is a fast, concurrent OSINT username search tool written in Go. It probes **300+** sites, colour-codes uncertain hits, and can also search breach databases (HudsonRock, ProxyNova, BreachDirectory).

This fork adds Gulf/Arabic platforms, stronger response matching, structured reports, a local web UI, and self-monitoring.

### Installation

```bash
go install github.com/ibnaleem/gosearch@latest
# or from this repo:
go build -o gosearch .
```

### Basic usage

```bash
gosearch -u [username]
gosearch -u [username] --no-false-positives
```

### New flags

| Flag | Description |
|------|-------------|
| `--output csv\|json\|html` | Write a structured report (HTML is a single sortable file) |
| `-o <dir>` | Directory for output files (default: current directory) |
| `--concurrency <n>` | Max parallel site probes (default: `20`) |
| `--rate-limit <duration>` | Polite delay between request starts (e.g. `50ms`) |
| `--timeout <duration>` | Default per-site timeout (default: `30s`) |
| `--retries <n>` | Retries with exponential backoff (default: `2`) |
| `--check-sites` | Test each site with a known-existing and a random username; report broken entries |
| `--watch <file>` | Re-check usernames listed in a file; print only **new/changed** findings |
| `--watch-state <file>` | Watch state JSON (default: `.gosearch-watch-state.json`) |

Examples:

```bash
gosearch -u alice --output html --concurrency 15 --rate-limit 50ms
gosearch --check-sites --concurrency 10 --rate-limit 100ms
gosearch --watch my-usernames.txt
```

`my-usernames.txt` is one username per line (`#` comments allowed).

### Web UI

```bash
gosearch serve --port 8080
```

Open `http://localhost:8080`, enter a username, watch results stream live (SSE), then download HTML/JSON/CSV.

### Accuracy / site data

Each site in `data.json` can use:

- Legacy `errorType`: `status_code`, `errorMsg`, `profilePresence`, `response_url`, `unknown`
- `present` / `absent`: body substrings that must appear / must not appear
- `response_url`: final redirect URL that means “not found”
- `timeout_ms`, `retries`, `category`, `check_username`

Local `data.json` is preferred (fork customizations are kept). Upstream is only downloaded if no local file exists.

Gulf/Arabic and popular platforms added or tagged include **Haraj**, **OpenSooq**, **Anghami**, **Dubizzle**, **Bayt**, plus **Snapchat**, **TikTok**, **Twitch**, **Threads**, **Kick**, and **Bluesky**.

> Existing site entries are never deleted without asking.

### Breach / OSINT extras

```bash
gosearch -u [USERNAME] -b [BREACHDIRECTORY-API-KEY] --no-false-positives
```

HudsonRock and ProxyNova run without an API key. Common TLDs are also checked for the username as a domain.

---

## العربية

### نظرة عامة

**GoSearch** أداة OSINT سريعة مكتوبة بلغة Go للبحث عن اسم مستخدم عبر **أكثر من 300** موقعًا، مع تمييز النتائج غير المؤكدة، ودعم قواعد بيانات التسريبات.

هذا الفرع يضيف منصات خليجية/عربية، مطابقة أدق للاستجابات، تقارير منظمة، واجهة ويب محلية، ومراقبة ذاتية لأسماءك.

### التثبيت

```bash
go install github.com/ibnaleem/gosearch@latest
# أو من هذا المستودع:
go build -o gosearch .
```

### الاستخدام الأساسي

```bash
gosearch -u [اسم_المستخدم]
gosearch -u [اسم_المستخدم] --no-false-positives
```

### الخيارات الجديدة

| الخيار | الوصف |
|--------|--------|
| `--output csv\|json\|html` | حفظ تقرير منظم (HTML ملف واحد مع جدول قابل للفرز) |
| `-o <مجلد>` | مجلد حفظ المخرجات |
| `--concurrency <n>` | أقصى عدد طلبات متوازية (افتراضي `20`) |
| `--rate-limit <مدة>` | تأخير مهذب بين الطلبات (مثل `50ms`) |
| `--timeout <مدة>` | مهلة كل موقع (افتراضي `30s`) |
| `--retries <n>` | إعادة المحاولة مع تراجع أسي |
| `--check-sites` | اختبار كل موقع بمستخدم موجود ومعروف وآخر عشوائي |
| `--watch <ملف>` | إعادة فحص قائمة أسماءك وطباعة التغييرات فقط |
| `--watch-state <ملف>` | ملف حالة المراقبة JSON |

أمثلة:

```bash
gosearch -u alice --output html --concurrency 15 --rate-limit 50ms
gosearch --check-sites
gosearch --watch my-usernames.txt
```

### واجهة الويب

```bash
gosearch serve --port 8080
```

افتح `http://localhost:8080`، أدخل اسم المستخدم، شاهد النتائج مباشرة (SSE)، ثم حمّل التقرير.

### الدقة وبيانات المواقع

يدعم `data.json` حقول `present` / `absent` وفحص رابط إعادة التوجيه النهائي، بالإضافة لأنواع الأخطاء السابقة. يُفضَّل الملف المحلي حتى تبقى إضافات المنصات الخليجية محفوظة.

### المساهمة

انظر [CONTRIBUTING.md](CONTRIBUTING.md). لا تحذف إدخالات مواقع موجودة دون مناقشة.

---

## Contributing

Please see [CONTRIBUTING.md](https://github.com/ibnaleem/gosearch/blob/main/CONTRIBUTING.md).

## LICENSE

This project is licensed under the GNU General Public License — see [LICENSE](LICENSE).

## Support

[![BuyMeACoffee](https://img.shields.io/badge/Buy%20Me%20a%20Coffee-ffdd00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black)](https://buymeacoffee.com/gosearch)
