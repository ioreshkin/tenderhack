package app

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed web/*
var webFiles embed.FS

func RunServe(ctx context.Context, cfg Config) error {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return err
	}
	if err := ensureDir(cfg.DocsDir); err != nil {
		return err
	}
	if err := LoadSearchIndex(ctx, pool); err != nil {
		return err
	}

	staticFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		return err
	}
	staticHandler := noStore(http.FileServer(http.FS(staticFS)))
	generatedHandler := noStore(http.StripPrefix("/generated/", http.FileServer(http.Dir(cfg.DocsDir))))

	mux := http.NewServeMux()
	mux.Handle("/generated/", generatedHandler)
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		resp, err := bootstrapData(r.Context(), pool, cfg)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/api/suggest", func(w http.ResponseWriter, r *http.Request) {
		resp, err := SuggestCTE(r.Context(), pool, r.URL.Query().Get("q"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		resp, err := SearchCTE(r.Context(), pool, r.URL.Query().Get("q"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/api/calculate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req CalculateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		resp, err := CalculateNMCK(r.Context(), pool, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/api/calculate/batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req BatchCalculateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		resp, err := CalculateBatchNMCK(r.Context(), pool, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
	mux.HandleFunc("/api/document", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req CalculateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		version, filePath, err := GeneratePDFDocument(r.Context(), pool, cfg, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"version":   version,
			"file_path": filePath,
			"file_url":  fileURL(filePath),
		})
	})
	mux.HandleFunc("/api/document/batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req BatchCalculateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		version, filePath, err := GenerateBatchPDFDocument(r.Context(), pool, cfg, req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"version":   version,
			"file_path": filePath,
			"file_url":  fileURL(filePath),
		})
	})
	mux.Handle("/", staticHandler)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("server listening on %s", cfg.HTTPAddr)
	return server.ListenAndServe()
}

func bootstrapData(ctx context.Context, pool *pgxpool.Pool, cfg Config) (bootstrapResponse, error) {
	var cteCount, contractCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM cte_items").Scan(&cteCount); err != nil {
		return bootstrapResponse{}, err
	}
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM contract_items").Scan(&contractCount); err != nil {
		return bootstrapResponse{}, err
	}
	rows, err := pool.Query(ctx, `
		SELECT region
		FROM (
			SELECT DISTINCT customer_region AS region FROM contract_items WHERE customer_region <> ''
			UNION
			SELECT DISTINCT supplier_region AS region FROM contract_items WHERE supplier_region <> ''
		) regions
	`)
	if err != nil {
		return bootstrapResponse{}, err
	}
	defer rows.Close()

	rawRegions := make([]string, 0, 128)
	for rows.Next() {
		var region string
		if err := rows.Scan(&region); err != nil {
			return bootstrapResponse{}, err
		}
		rawRegions = append(rawRegions, region)
	}
	if err := rows.Err(); err != nil {
		return bootstrapResponse{}, err
	}
	regions := canonicalRegionList(rawRegions)
	sort.Strings(regions)

	recentDocs, err := recentDocuments(ctx, pool, cfg)
	if err != nil {
		return bootstrapResponse{}, err
	}

	return bootstrapResponse{
		Regions:         regions,
		Imported:        cteCount > 0 && contractCount > 0,
		CTECount:        cteCount,
		ContractCount:   contractCount,
		RecentDocuments: recentDocs,
	}, nil
}

func recentDocuments(ctx context.Context, pool *pgxpool.Pool, cfg Config) ([]RecentDocument, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			doc_type,
			cte_id,
			name,
			category,
			manufacturer,
			region,
			months_back,
			version,
			summary,
			batch_key,
			item_count,
			file_path,
			created_at
		FROM (
			SELECT
				'single'::text AS doc_type,
				d.cte_id,
				c.name,
				c.category,
				c.manufacturer,
				d.region,
				d.months_back,
				d.version,
				d.summary,
				''::text AS batch_key,
				1::integer AS item_count,
				d.file_path,
				d.created_at
			FROM doc_versions d
			JOIN cte_items c ON c.id = d.cte_id
			UNION ALL
			SELECT
				'batch'::text AS doc_type,
				0::bigint AS cte_id,
				b.batch_name AS name,
				''::text AS category,
				''::text AS manufacturer,
				b.region,
				b.months_back,
				b.version,
				b.summary,
				b.batch_key,
				b.item_count,
				b.file_path,
				b.created_at
			FROM batch_doc_versions b
		) docs
		ORDER BY created_at DESC
		LIMIT 12
	`)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	result := make([]RecentDocument, 0, 12)
	for rows.Next() {
		var item RecentDocument
		if err := rows.Scan(
			&item.DocType,
			&item.CTEID,
			&item.Name,
			&item.Category,
			&item.Manufacturer,
			&item.Region,
			&item.MonthsBack,
			&item.Version,
			&item.Summary,
			&item.BatchKey,
			&item.ItemCount,
			&item.FilePath,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if item.FilePath == "" {
			candidate := ""
			if item.DocType == "batch" {
				candidate = filepath.Join(cfg.DocsDir, fmt.Sprintf("nmck_batch_%s_v%d.pdf", item.BatchKey, item.Version))
			} else {
				candidate = filepath.Join(cfg.DocsDir, fmt.Sprintf("nmck_cte_%d_v%d.pdf", item.CTEID, item.Version))
			}
			if _, err := os.Stat(candidate); err == nil {
				item.FilePath = candidate
			}
		}
		item.FileURL = fileURL(item.FilePath)
		result = append(result, item)
	}
	return result, rows.Err()
}

func fileURL(filePath string) string {
	if filePath == "" {
		return ""
	}
	return "/generated/" + url.PathEscape(filepath.Base(filePath))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, os.ErrNotExist) {
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		next.ServeHTTP(w, r)
	})
}
