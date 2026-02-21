package scanning

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Init(ctx context.Context) error {
	// 1. Scan Results table
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS scan_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  target TEXT NOT NULL,
  source TEXT NOT NULL,
  scanned_at INTEGER NOT NULL,
  critical INTEGER NOT NULL,
  high INTEGER NOT NULL,
  medium INTEGER NOT NULL,
  low INTEGER NOT NULL,
  unknown INTEGER NOT NULL,
  raw_json TEXT NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("create scan_results table: %w", err)
	}

	// 2. Malware results table (v0.5.0)
	_, err = s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS malware_scan_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  target TEXT NOT NULL,
  source TEXT NOT NULL,
  scanned_at INTEGER NOT NULL,
  infected INTEGER NOT NULL,
  threats_found TEXT NOT NULL,
  raw_output TEXT NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("create malware_scan_results table: %w", err)
	}

	// 3. Scan jobs table (v0.5.0)
	_, err = s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS scan_jobs (
  job_id TEXT PRIMARY KEY,
  target TEXT NOT NULL,
  type TEXT NOT NULL,
  status TEXT NOT NULL,
  source TEXT NOT NULL,
  progress INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  started_at INTEGER NOT NULL,
  completed_at INTEGER NOT NULL DEFAULT 0
);
`)
	if err != nil {
		return fmt.Errorf("create scan_jobs table: %w", err)
	}

	if err := s.ensureColumn(ctx, "scan_jobs", "progress", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("ensure scan_jobs.progress: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS idx_scan_results_target_scanned ON scan_results(target, scanned_at DESC);
CREATE INDEX IF NOT EXISTS idx_malware_results_target_scanned ON malware_scan_results(target, scanned_at DESC);
CREATE INDEX IF NOT EXISTS idx_scan_jobs_type_started ON scan_jobs(type, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_scan_jobs_status_started ON scan_jobs(status, started_at DESC);
`); err != nil {
		return fmt.Errorf("init scanning indexes: %w", err)
	}

	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table, column, ddl string) error {
	query := fmt.Sprintf("SELECT 1 FROM pragma_table_info('%s') WHERE name=? LIMIT 1", table)
	var exists int
	err := s.db.QueryRowContext(ctx, query, column).Scan(&exists)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, ddl)
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return nil
		}
		return err
	}
	return nil
}

func (s *Store) SaveResult(ctx context.Context, r Result) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO scan_results(target, source, scanned_at, critical, high, medium, low, unknown, raw_json)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
`, r.Target, r.Source, r.Scanned, r.Critical, r.High, r.Medium, r.Low, r.Unknown, r.RawJSON)
	if err != nil {
		return fmt.Errorf("insert scan result: %w", err)
	}
	return nil
}

func (s *Store) SaveMalwareResult(ctx context.Context, r MalwareResult) error {
	threats, _ := json.Marshal(r.FoundThreats)
	infected := 0
	if r.Infected {
		infected = 1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO malware_scan_results(target, source, scanned_at, infected, threats_found, raw_output)
VALUES(?, ?, ?, ?, ?, ?)
`, r.Target, r.Source, r.ScannedAt, infected, string(threats), r.RawOutput)
	if err != nil {
		return fmt.Errorf("insert malware scan result: %w", err)
	}
	return nil
}

func (s *Store) MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error) {
	query := `SELECT target, source, scanned_at, infected, threats_found FROM malware_scan_results`
	var args []any
	if target != "" {
		query += ` WHERE target = ?`
		args = append(args, target)
	}
	query += ` ORDER BY scanned_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query malware summaries: %w", err)
	}
	defer rows.Close()

	var summaries []gen.MalwareScanSummary
	for rows.Next() {
		var sm gen.MalwareScanSummary
		var infected int
		var threatsRaw string
		if err := rows.Scan(&sm.Target, &sm.Source, &sm.ScannedAt, &infected, &threatsRaw); err != nil {
			return nil, fmt.Errorf("scan malware summary: %w", err)
		}
		sm.Infected = infected == 1
		_ = json.Unmarshal([]byte(threatsRaw), &sm.ThreatsFound)
		summaries = append(summaries, sm)
	}
	return summaries, nil
}

func (s *Store) MalwareSummariesByPrefix(ctx context.Context, prefix string) ([]gen.MalwareScanSummary, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return []gen.MalwareScanSummary{}, nil
	}
	query := `SELECT target, source, scanned_at, infected, threats_found
FROM malware_scan_results
WHERE target = ? OR target LIKE ? OR target LIKE ?
ORDER BY scanned_at DESC`
	rows, err := s.db.QueryContext(ctx, query, prefix, prefix+":%", prefix+"%:%")
	if err != nil {
		return nil, fmt.Errorf("query malware summaries by prefix: %w", err)
	}
	defer rows.Close()

	var summaries []gen.MalwareScanSummary
	for rows.Next() {
		var sm gen.MalwareScanSummary
		var infected int
		var threatsRaw string
		if err := rows.Scan(&sm.Target, &sm.Source, &sm.ScannedAt, &infected, &threatsRaw); err != nil {
			return nil, fmt.Errorf("scan malware summary: %w", err)
		}
		sm.Infected = infected == 1
		_ = json.Unmarshal([]byte(threatsRaw), &sm.ThreatsFound)
		summaries = append(summaries, sm)
	}
	return summaries, nil
}

func (s *Store) MalwareDetails(ctx context.Context, target, prefix string, limit int) ([]gen.MalwareScanDetail, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}

	query := `SELECT target, source, scanned_at, infected, threats_found, raw_output FROM malware_scan_results`
	var (
		clauses []string
		args    []any
	)
	if target != "" {
		clauses = append(clauses, "target = ?")
		args = append(args, target)
	}
	if prefix != "" {
		clauses = append(clauses, "(target = ? OR target LIKE ? OR target LIKE ?)")
		args = append(args, prefix, prefix+":%", prefix+"%:%")
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY scanned_at DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query malware details: %w", err)
	}
	defer rows.Close()

	details := make([]gen.MalwareScanDetail, 0, limit)
	for rows.Next() {
		var (
			detail     gen.MalwareScanDetail
			infected   int
			threatsRaw string
			rawOutput  string
		)
		if err := rows.Scan(&detail.Target, &detail.Source, &detail.ScannedAt, &infected, &threatsRaw, &rawOutput); err != nil {
			return nil, fmt.Errorf("scan malware detail: %w", err)
		}
		detail.Infected = infected == 1
		detail.RawOutput = rawOutput
		if strings.TrimSpace(threatsRaw) != "" {
			if err := json.Unmarshal([]byte(threatsRaw), &detail.ThreatsFound); err != nil {
				detail.ParseError = fmt.Sprintf("parse threats json: %v", err)
			}
		}
		matches := parseClamThreatDetails(rawOutput)
		detail.ThreatDetails = make([]gen.MalwareThreatDetail, 0, len(matches))
		for _, m := range matches {
			detail.ThreatDetails = append(detail.ThreatDetails, gen.MalwareThreatDetail{
				Path:      m.Path,
				Signature: m.Signature,
			})
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate malware details: %w", err)
	}
	return details, nil
}

func (s *Store) CreateJob(ctx context.Context, job gen.ScanJobStatus, scanType string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO scan_jobs(job_id, target, type, status, source, started_at)
VALUES(?, ?, ?, ?, ?, ?)
`, job.JobID, job.Target, scanType, job.Status, job.Source, job.StartedAt)
	if err != nil {
		return fmt.Errorf("create scan job: %w", err)
	}
	return nil
}

func (s *Store) UpdateJob(ctx context.Context, jobID, status, errMsg string, completedAt int64) error {
	progress := 0
	if status == "completed" {
		progress = 100
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE scan_jobs SET status=?, error=?, completed_at=?, progress=MAX(progress, ?) WHERE job_id=?
`, status, errMsg, completedAt, progress, jobID)
	if err != nil {
		return fmt.Errorf("update scan job: %w", err)
	}
	return nil
}

func (s *Store) UpdateJobProgress(ctx context.Context, jobID, status string, progress int) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE scan_jobs SET status=?, progress=? WHERE job_id=?
`, status, progress, jobID)
	if err != nil {
		return fmt.Errorf("update scan job progress: %w", err)
	}
	return nil
}

func (s *Store) GetJob(ctx context.Context, jobID string) (*gen.ScanJobStatus, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT job_id, target, status, source, progress, error, started_at, completed_at
FROM scan_jobs WHERE job_id=?
`, jobID)

	var job gen.ScanJobStatus
	if err := row.Scan(&job.JobID, &job.Target, &job.Status, &job.Source, &job.Progress, &job.Error, &job.StartedAt, &job.CompletedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("read scan job: %w", err)
	}
	return &job, nil
}

func (s *Store) ListJobs(ctx context.Context, scanType, targetPrefix string, limit int) ([]gen.ScanJobStatus, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	query := `
SELECT job_id, target, status, source, progress, error, started_at, completed_at
FROM scan_jobs
`
	var (
		clauses []string
		args    []any
	)
	if t := strings.TrimSpace(scanType); t != "" {
		clauses = append(clauses, "type = ?")
		args = append(args, t)
	}
	if p := strings.TrimSpace(targetPrefix); p != "" {
		clauses = append(clauses, "target LIKE ?")
		args = append(args, p+"%")
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list scan jobs: %w", err)
	}
	defer rows.Close()

	out := make([]gen.ScanJobStatus, 0, limit)
	for rows.Next() {
		var job gen.ScanJobStatus
		if err := rows.Scan(&job.JobID, &job.Target, &job.Status, &job.Source, &job.Progress, &job.Error, &job.StartedAt, &job.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan scan job row: %w", err)
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scan jobs: %w", err)
	}
	return out, nil
}

func (s *Store) PruneVulnerabilityResults(ctx context.Context, olderThan int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM scan_results WHERE scanned_at < ?", olderThan)
	if err != nil {
		return 0, fmt.Errorf("prune vulnerability scan results: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("vulnerability scan rows affected: %w", err)
	}
	return rows, nil
}

func (s *Store) PruneMalwareResults(ctx context.Context, olderThan int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM malware_scan_results WHERE scanned_at < ?", olderThan)
	if err != nil {
		return 0, fmt.Errorf("prune malware scan results: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("malware scan rows affected: %w", err)
	}
	return rows, nil
}

func (s *Store) PruneScanJobs(ctx context.Context, olderThan int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM scan_jobs WHERE started_at < ?", olderThan)
	if err != nil {
		return 0, fmt.Errorf("prune scan jobs: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("scan job rows affected: %w", err)
	}
	return rows, nil
}

func (s *Store) MarkRunningJobsFailed(ctx context.Context, reason string) (int64, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "scan interrupted by HarborWatch restart"
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE scan_jobs
SET status = 'failed', error = ?, completed_at = ?
WHERE status = 'running'
`, reason, time.Now().UTC().Unix())
	if err != nil {
		return 0, fmt.Errorf("mark running scan jobs failed: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected running scan jobs: %w", err)
	}
	return n, nil
}

func (s *Store) LatestSummary(ctx context.Context) (*gen.ScanSummary, error) {
	return s.LatestSummaryForTarget(ctx, "")
}

func (s *Store) LatestSummaryForTarget(ctx context.Context, target string) (*gen.ScanSummary, error) {
	query := `
SELECT target, source, scanned_at, critical, high, medium, low, unknown
FROM scan_results
`
	var args []any
	if target != "" {
		query += " WHERE target = ?"
		args = append(args, target)
	}
	query += " ORDER BY scanned_at DESC, id DESC LIMIT 1"

	row := s.db.QueryRowContext(ctx, query, args...)

	var summary gen.ScanSummary
	if err := row.Scan(
		&summary.Target,
		&summary.Source,
		&summary.ScannedAt,
		&summary.Critical,
		&summary.High,
		&summary.Medium,
		&summary.Low,
		&summary.Unknown,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("load latest summary: %w", err)
	}

	summary.Total = summary.Critical + summary.High + summary.Medium + summary.Low + summary.Unknown
	summary.RiskScore = riskScore(summary)
	return &summary, nil
}

func (s *Store) LatestDetailsForTarget(ctx context.Context, target string) (*gen.TrivyScanDetails, error) {
	query := `
SELECT target, source, scanned_at, critical, high, medium, low, unknown, raw_json
FROM scan_results
`
	var args []any
	if target != "" {
		query += " WHERE target = ?"
		args = append(args, target)
	}
	query += " ORDER BY scanned_at DESC, id DESC LIMIT 1"

	row := s.db.QueryRowContext(ctx, query, args...)
	var (
		resTarget string
		source    string
		scannedAt int64
		critical  int
		high      int
		medium    int
		low       int
		unknown   int
		rawJSON   string
	)
	if err := row.Scan(&resTarget, &source, &scannedAt, &critical, &high, &medium, &low, &unknown, &rawJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("load latest details: %w", err)
	}

	summary := gen.ScanSummary{
		Target:    resTarget,
		Source:    source,
		ScannedAt: scannedAt,
		Critical:  critical,
		High:      high,
		Medium:    medium,
		Low:       low,
		Unknown:   unknown,
	}
	summary.Total = summary.Critical + summary.High + summary.Medium + summary.Low + summary.Unknown
	summary.RiskScore = riskScore(summary)

	details := &gen.TrivyScanDetails{
		Target:    resTarget,
		Source:    source,
		ScannedAt: scannedAt,
		Summary:   summary,
		RawJSON:   rawJSON,
		Results:   []gen.TrivyResultGroup{},
	}
	if strings.TrimSpace(rawJSON) == "" {
		details.ParseError = "raw Trivy JSON was empty"
		return details, nil
	}

	var report trivyRawReport
	if err := json.Unmarshal([]byte(rawJSON), &report); err != nil {
		details.ParseError = fmt.Sprintf("parse raw Trivy JSON: %v", err)
		return details, nil
	}

	for _, result := range report.Results {
		group := gen.TrivyResultGroup{
			Type:            result.Type,
			Target:          result.Target,
			Class:           result.Class,
			Vulnerabilities: []gen.TrivyVulnerability{},
		}
		for _, vuln := range result.Vulnerabilities {
			score, source := trivyBestCVSS(vuln.CVSS)
			refs := make([]string, 0, len(vuln.References))
			for _, ref := range vuln.References {
				ref = strings.TrimSpace(ref)
				if ref != "" {
					refs = append(refs, ref)
				}
			}
			group.Vulnerabilities = append(group.Vulnerabilities, gen.TrivyVulnerability{
				ID:               vuln.VulnerabilityID,
				PkgName:          vuln.PkgName,
				InstalledVersion: vuln.InstalledVersion,
				FixedVersion:     vuln.FixedVersion,
				Severity:         vuln.Severity,
				Title:            vuln.Title,
				Description:      vuln.Description,
				PrimaryURL:       vuln.PrimaryURL,
				CVSSScore:        score,
				CVSSSource:       source,
				PublishedDate:    vuln.PublishedDate,
				LastModifiedDate: vuln.LastModifiedDate,
				References:       refs,
			})
		}
		details.Results = append(details.Results, group)
	}

	return details, nil
}

type trivyRawReport struct {
	Results []trivyRawResult `json:"Results"`
}

type trivyRawResult struct {
	Type            string                  `json:"Type"`
	Target          string                  `json:"Target"`
	Class           string                  `json:"Class"`
	Vulnerabilities []trivyRawVulnerability `json:"Vulnerabilities"`
}

type trivyRawVulnerability struct {
	VulnerabilityID  string                  `json:"VulnerabilityID"`
	PkgName          string                  `json:"PkgName"`
	InstalledVersion string                  `json:"InstalledVersion"`
	FixedVersion     string                  `json:"FixedVersion"`
	Severity         string                  `json:"Severity"`
	Title            string                  `json:"Title"`
	Description      string                  `json:"Description"`
	PrimaryURL       string                  `json:"PrimaryURL"`
	PublishedDate    string                  `json:"PublishedDate"`
	LastModifiedDate string                  `json:"LastModifiedDate"`
	References       []string                `json:"References"`
	CVSS             map[string]trivyRawCVSS `json:"CVSS"`
}

type trivyRawCVSS struct {
	V3Score float64 `json:"V3Score"`
	V2Score float64 `json:"V2Score"`
}

func trivyBestCVSS(cvss map[string]trivyRawCVSS) (float64, string) {
	bestScore := 0.0
	bestSource := ""
	for source, score := range cvss {
		candidate := score.V3Score
		if candidate <= 0 {
			candidate = score.V2Score
		}
		if candidate > bestScore {
			bestScore = candidate
			bestSource = source
		}
	}
	return bestScore, bestSource
}

func riskScore(s gen.ScanSummary) int {
	score := (s.Critical * 10) + (s.High * 7) + (s.Medium * 4) + (s.Low * 2) + s.Unknown
	if score > 100 {
		return 100
	}
	return score
}
