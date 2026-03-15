package app

import (
	"context"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func GeneratePDFDocument(ctx context.Context, pool *pgxpool.Pool, cfg Config, req CalculateRequest) (int, string, error) {
	result, err := CalculateNMCK(ctx, pool, req)
	if err != nil {
		return 0, "", err
	}
	if err := ensureDir(cfg.DocsDir); err != nil {
		return 0, "", err
	}
	body, err := buildSingleDocumentHTML(result)
	if err != nil {
		return 0, "", err
	}

	var version int
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM doc_versions
		WHERE cte_id = $1
	`, req.CTEID).Scan(&version)
	if err != nil {
		return 0, "", err
	}

	htmlPath, err := writeTempHTML(cfg.DocsDir, fmt.Sprintf("nmck_cte_%d_v%d_*.source.html", req.CTEID, version), body)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = os.Remove(htmlPath) }()

	pdfPath := filepath.Join(cfg.DocsDir, fmt.Sprintf("nmck_cte_%d_v%d.pdf", req.CTEID, version))
	if err := renderPDF(ctx, htmlPath, pdfPath); err != nil {
		return 0, "", err
	}

	summary := fmt.Sprintf("НМЦК %s, диапазон %s - %s, закупок в документе %d", formatMoneyValue(result.Summary.NMCKWeightedMean), formatMoneyValue(result.Summary.PriceRangeMin), formatMoneyValue(result.Summary.PriceRangeMax), result.Summary.DocumentContracts)
	if _, err := pool.Exec(ctx, `
		INSERT INTO doc_versions(cte_id, version, region, months_back, params_json, summary, file_path, body_html)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8)
	`, req.CTEID, version, req.Region, req.MonthsBack, mustJSON(req), summary, pdfPath, body); err != nil {
		return 0, "", err
	}

	return version, pdfPath, nil
}

func GenerateBatchPDFDocument(ctx context.Context, pool *pgxpool.Pool, cfg Config, req BatchCalculateRequest) (int, string, error) {
	result, err := CalculateBatchNMCK(ctx, pool, req)
	if err != nil {
		return 0, "", err
	}
	if err := ensureDir(cfg.DocsDir); err != nil {
		return 0, "", err
	}
	body, err := buildBatchDocumentHTML(result)
	if err != nil {
		return 0, "", err
	}

	key := batchKey(req)
	var version int
	err = pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM batch_doc_versions
		WHERE batch_key = $1
	`, key).Scan(&version)
	if err != nil {
		return 0, "", err
	}

	htmlPath, err := writeTempHTML(cfg.DocsDir, fmt.Sprintf("nmck_batch_%s_v%d_*.source.html", key, version), body)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = os.Remove(htmlPath) }()

	pdfPath := filepath.Join(cfg.DocsDir, fmt.Sprintf("nmck_batch_%s_v%d.pdf", key, version))
	if err := renderPDF(ctx, htmlPath, pdfPath); err != nil {
		return 0, "", err
	}

	summary := fmt.Sprintf("Лот %q, позиций %d, итог %s", result.Summary.BatchName, result.Summary.ItemCount, formatMoneyValue(result.Summary.GrandTotal))
	if _, err := pool.Exec(ctx, `
		INSERT INTO batch_doc_versions(batch_key, batch_name, version, region, months_back, item_count, params_json, summary, file_path, body_html)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10)
	`, key, result.Summary.BatchName, version, req.Region, req.MonthsBack, len(result.Items), mustJSON(req), summary, pdfPath, body); err != nil {
		return 0, "", err
	}

	return version, pdfPath, nil
}

func writeTempHTML(dir, pattern, body string) (string, error) {
	htmlFile, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	if _, err := htmlFile.WriteString(body); err != nil {
		_ = htmlFile.Close()
		return "", err
	}
	if err := htmlFile.Close(); err != nil {
		return "", err
	}
	return htmlFile.Name(), nil
}

func documentTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"inc":                 func(v int) int { return v + 1 },
		"money":               formatMoneyValue,
		"vatSummaryLabel":     vatSummaryLabel,
		"timeWeightModeLabel": timeWeightModeLabel,
		"percent": func(v float64) string {
			return fmt.Sprintf("%.0f%%", v*100)
		},
		"date": func(v time.Time) string {
			if v.IsZero() {
				return "-"
			}
			return v.Format("02.01.2006")
		},
		"lineTotal": func(unitPrice, quantity float64) float64 { return unitPrice * quantity },
	}
}

func formatMoneyValue(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

const documentBaseStyles = `
  <style>
    @page { size: A4 landscape; margin: 12mm; }
    :root {
      --ink: #0f172a;
      --muted: #475569;
      --line: #cbd5e1;
      --soft: #f8fafc;
      --soft-2: #eef2f7;
      --accent: #7f1d1d;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Segoe UI", Arial, sans-serif;
      color: var(--ink);
      font-size: 10.5px;
      line-height: 1.35;
    }
    h1, h2, h3, p { margin: 0; }
    .doc { display: grid; gap: 10px; }
    .title {
      text-align: center;
      display: grid;
      gap: 4px;
      margin-top: 4px;
    }
    .title .label {
      color: var(--accent);
      text-transform: uppercase;
      letter-spacing: 0.14em;
      font-size: 9px;
      font-weight: 700;
    }
    .title h1 {
      font-size: 18px;
      line-height: 1.1;
    }
    .title p {
      color: var(--muted);
      font-size: 11px;
    }
    .section-title {
      font-size: 11px;
      font-weight: 700;
      padding: 6px 8px;
      background: var(--soft-2);
      border: 1px solid var(--line);
    }
    .meta-grid, .summary-grid, .footer-grid {
      display: grid;
      gap: 10px;
    }
    .meta-grid { grid-template-columns: 1.3fr 1fr; }
    .summary-grid { grid-template-columns: repeat(4, 1fr); gap: 8px; }
    .footer-grid { grid-template-columns: 1fr 1fr; }
    .meta-box, .summary-card, .signature-box, .step {
      border: 1px solid var(--line);
      background: #fff;
    }
    .summary-card, .signature-box, .step { padding: 8px; }
    .summary-card { background: var(--soft); min-height: 62px; }
    .summary-card .label {
      display: block;
      color: var(--muted);
      font-size: 9px;
      text-transform: uppercase;
      letter-spacing: 0.08em;
      margin-bottom: 4px;
    }
    .summary-card strong { font-size: 15px; line-height: 1.1; }
    table { width: 100%; border-collapse: collapse; }
    th, td { border: 1px solid var(--line); padding: 5px 6px; vertical-align: top; }
    th { background: var(--soft-2); font-weight: 700; }
    tfoot td { font-weight: 700; background: var(--soft); }
    .meta-box table td:first-child {
      width: 38%;
      background: var(--soft);
      font-weight: 600;
    }
    .steps { display: grid; gap: 6px; }
    .step h3 { font-size: 10.5px; margin-bottom: 3px; }
    .muted { color: var(--muted); }
    .right { text-align: right; }
    .center { text-align: center; }
    .small { font-size: 9.2px; }
    .notes { display: grid; gap: 6px; color: var(--muted); font-size: 9.4px; }
    .page-break { page-break-before: always; }
  </style>
`

func renderPDF(ctx context.Context, htmlPath, pdfPath string) error {
	absHTML, err := filepath.Abs(htmlPath)
	if err != nil {
		return err
	}
	absPDF, err := filepath.Abs(pdfPath)
	if err != nil {
		return err
	}

	fileURL := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(absHTML)}).String()
	profileDir, err := os.MkdirTemp(filepath.Dir(absPDF), ".tenderhack-pdf-browser-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(profileDir) }()

	var lastErr error
	for _, browserPath := range detectPDFBrowsers() {
		cmdCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		args := []string{
			"--headless=new",
			"--disable-gpu",
			"--no-sandbox",
			"--no-first-run",
			"--no-default-browser-check",
			"--disable-extensions",
			"--disable-background-networking",
			"--disable-crashpad",
			"--disable-crash-reporter",
			"--disable-breakpad",
			"--disable-features=Crashpad",
			"--disable-dev-shm-usage",
			"--disable-sync",
			"--metrics-recording-only",
			"--allow-file-access-from-files",
			"--user-data-dir=" + profileDir,
			"--print-to-pdf=" + absPDF,
			"--no-pdf-header-footer",
			"--window-size=1440,2200",
			fileURL,
		}
		cmd := exec.CommandContext(cmdCtx, browserPath, args...)
		output, err := cmd.CombinedOutput()
		cancel()
		if err == nil {
			if _, statErr := os.Stat(absPDF); statErr == nil {
				return nil
			}
			lastErr = fmt.Errorf("pdf was not created by %s", filepath.Base(browserPath))
			continue
		}
		lastErr = fmt.Errorf("render pdf via %s: %w: %s", filepath.Base(browserPath), err, cleanCell(string(output)))
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("pdf browser was not found; set PDF_BROWSER or install Google Chrome/Microsoft Edge")
}

func detectPDFBrowsers() []string {
	candidates := []string{}
	if envPath := strings.TrimSpace(os.Getenv("PDF_BROWSER")); envPath != "" {
		candidates = append(candidates, envPath)
	}
	candidates = append(candidates,
		`/usr/bin/chromium`,
		`/usr/bin/chromium-browser`,
		`/usr/bin/google-chrome`,
		`/usr/bin/google-chrome-stable`,
		`/usr/bin/microsoft-edge`,
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	)
	found := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			found = append(found, candidate)
		}
	}
	return found
}
