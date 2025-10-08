package server

import (
	"fmt"
	"html/template"
	"net/http"
	"time"
)

// handleDashboard serves the main dashboard HTML page
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch data for the dashboard
	var jobs []JobSummary
	var runs []RunRecord
	var stats *StatsResponse

	if s.scheduler != nil {
		fetchedJobs, err := s.scheduler.GetJobs(ctx)
		if err != nil {
			s.logger.Error("failed to get jobs for dashboard", "error", err)
		} else {
			jobs = fetchedJobs
		}
	}

	if s.store != nil {
		fetchedRuns, err := s.store.GetRuns(ctx, nil, 20)
		if err != nil {
			s.logger.Error("failed to get runs for dashboard", "error", err)
		} else {
			runs = fetchedRuns
		}

		fetchedStats, err := s.store.GetStats(ctx)
		if err != nil {
			s.logger.Error("failed to get stats for dashboard", "error", err)
		} else {
			stats = fetchedStats
		}
	}

	// Prepare template data
	data := DashboardData{
		Title:   "Jobster Dashboard",
		Jobs:    jobs,
		Runs:    runs,
		Stats:   stats,
		Version: version,
		Uptime:  s.Uptime(),
	}

	// Render template
	tmpl := template.Must(template.New("dashboard").Funcs(templateFuncs).Parse(dashboardTemplate))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := tmpl.Execute(w, data); err != nil {
		s.logger.Error("failed to render dashboard template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleJobDetail serves the job detail page
func (s *Server) handleJobDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID := r.PathValue("id")

	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	// Fetch job details
	var job *JobSummary
	var runs []RunRecord

	if s.scheduler != nil {
		fetchedJob, err := s.scheduler.GetJob(ctx, jobID)
		if err != nil {
			s.logger.Error("failed to get job for detail page", "job_id", jobID, "error", err)
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		job = fetchedJob
	}

	if s.store != nil {
		fetchedRuns, err := s.store.GetRuns(ctx, &jobID, 50)
		if err != nil {
			s.logger.Error("failed to get runs for job detail", "job_id", jobID, "error", err)
		} else {
			runs = fetchedRuns
		}
	}

	if job == nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	// Prepare template data
	data := JobDetailData{
		Title: "Job: " + jobID,
		Job:   job,
		Runs:  runs,
	}

	// Render template
	tmpl := template.Must(template.New("jobdetail").Funcs(templateFuncs).Parse(jobDetailTemplate))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := tmpl.Execute(w, data); err != nil {
		s.logger.Error("failed to render job detail template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// DashboardData holds data for the dashboard template
type DashboardData struct {
	Title   string
	Jobs    []JobSummary
	Runs    []RunRecord
	Stats   *StatsResponse
	Version string
	Uptime  string
}

// JobDetailData holds data for the job detail template
type JobDetailData struct {
	Title string
	Job   *JobSummary
	Runs  []RunRecord
}

// templateFuncs provides custom template functions
var templateFuncs = template.FuncMap{
	"formatTime": func(t *time.Time) string {
		if t == nil {
			return "N/A"
		}
		return t.Format("2006-01-02 15:04:05")
	},
	"formatDuration": func(ms float64) string {
		duration := time.Duration(ms) * time.Millisecond
		if duration < time.Second {
			return duration.String()
		}
		return duration.Round(time.Millisecond).String()
	},
	"statusBadge": func(status interface{}) template.HTML {
		var s string
		switch v := status.(type) {
		case *string:
			if v == nil {
				return template.HTML(`<span class="badge badge-secondary">unknown</span>`)
			}
			s = *v
		case string:
			s = v
		default:
			return template.HTML(`<span class="badge badge-secondary">unknown</span>`)
		}

		switch s {
		case "success":
			return template.HTML(`<span class="badge badge-success">success</span>`)
		case "failure":
			return template.HTML(`<span class="badge badge-danger">failure</span>`)
		case "running":
			return template.HTML(`<span class="badge badge-info">running</span>`)
		default:
			return template.HTML(`<span class="badge badge-secondary">` + template.HTMLEscapeString(s) + `</span>`)
		}
	},
	"exitCodeBadge": func(code int) template.HTML {
		if code == 0 {
			return template.HTML(`<span class="badge badge-success">0</span>`)
		}
		return template.HTML(`<span class="badge badge-danger">` + template.HTMLEscapeString(fmt.Sprintf("%d", code)) + `</span>`)
	},
	"truncate": func(s string, max int) string {
		if len(s) <= max {
			return s
		}
		return s[:max] + "..."
	},
}

// dashboardTemplate is the main dashboard HTML template
const dashboardTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; line-height: 1.6; }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        header { background: #2c3e50; color: white; padding: 20px 0; margin-bottom: 30px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        header h1 { font-size: 28px; margin-bottom: 5px; }
        header .meta { font-size: 14px; opacity: 0.8; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 30px; }
        .stat-card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .stat-card h3 { font-size: 14px; color: #7f8c8d; margin-bottom: 8px; text-transform: uppercase; }
        .stat-card .value { font-size: 32px; font-weight: bold; color: #2c3e50; }
        .section { background: white; padding: 25px; border-radius: 8px; margin-bottom: 30px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .section h2 { font-size: 20px; margin-bottom: 20px; color: #2c3e50; border-bottom: 2px solid #3498db; padding-bottom: 10px; }
        table { width: 100%; border-collapse: collapse; }
        th { background: #f8f9fa; text-align: left; padding: 12px; font-weight: 600; border-bottom: 2px solid #dee2e6; }
        td { padding: 12px; border-bottom: 1px solid #dee2e6; }
        tr:hover { background: #f8f9fa; }
        .badge { display: inline-block; padding: 4px 8px; border-radius: 4px; font-size: 12px; font-weight: 600; text-transform: uppercase; }
        .badge-success { background: #d4edda; color: #155724; }
        .badge-danger { background: #f8d7da; color: #721c24; }
        .badge-info { background: #d1ecf1; color: #0c5460; }
        .badge-secondary { background: #e2e3e5; color: #383d41; }
        .empty { text-align: center; padding: 40px; color: #7f8c8d; }
        a { color: #3498db; text-decoration: none; }
        a:hover { text-decoration: underline; }
        code { background: #f8f9fa; padding: 2px 6px; border-radius: 3px; font-family: monospace; font-size: 13px; }
    </style>
</head>
<body>
    <header>
        <div class="container">
            <h1>{{.Title}}</h1>
            <div class="meta">Version: {{.Version}} | Uptime: {{.Uptime}}</div>
        </div>
    </header>

    <div class="container">
        {{if .Stats}}
        <div class="stats">
            <div class="stat-card">
                <h3>Total Jobs</h3>
                <div class="value">{{.Stats.TotalJobs}}</div>
            </div>
            <div class="stat-card">
                <h3>Total Runs</h3>
                <div class="value">{{.Stats.TotalRuns}}</div>
            </div>
            <div class="stat-card">
                <h3>Success Rate</h3>
                <div class="value">{{if gt .Stats.TotalRuns 0}}{{printf "%.1f%%" (mul (div (float64 .Stats.SuccessCount) (float64 .Stats.TotalRuns)) 100)}}{{else}}N/A{{end}}</div>
            </div>
            <div class="stat-card">
                <h3>Active Jobs</h3>
                <div class="value">{{.Stats.ActiveJobs}}</div>
            </div>
        </div>
        {{end}}

        <div class="section">
            <h2>Jobs ({{len .Jobs}})</h2>
            {{if .Jobs}}
            <table>
                <thead>
                    <tr>
                        <th>Job ID</th>
                        <th>Schedule</th>
                        <th>Command</th>
                        <th>Last Status</th>
                        <th>Last Run</th>
                        <th>Next Run</th>
                        <th>Success/Fail</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Jobs}}
                    <tr>
                        <td><a href="/jobs/{{.ID}}">{{.ID}}</a></td>
                        <td><code>{{.Schedule}}</code></td>
                        <td><code>{{truncate .Command 50}}</code></td>
                        <td>{{statusBadge .LastStatus}}</td>
                        <td>{{formatTime .LastRunTime}}</td>
                        <td>{{formatTime .NextRunTime}}</td>
                        <td>{{.SuccessCount}} / {{.FailureCount}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{else}}
            <div class="empty">No jobs configured</div>
            {{end}}
        </div>

        <div class="section">
            <h2>Recent Runs ({{len .Runs}})</h2>
            {{if .Runs}}
            <table>
                <thead>
                    <tr>
                        <th>Run ID</th>
                        <th>Job ID</th>
                        <th>Start Time</th>
                        <th>Duration</th>
                        <th>Exit Code</th>
                        <th>Status</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Runs}}
                    <tr>
                        <td><code>{{truncate .RunID 12}}</code></td>
                        <td><a href="/jobs/{{.JobID}}">{{.JobID}}</a></td>
                        <td>{{.StartTime.Format "2006-01-02 15:04:05"}}</td>
                        <td>{{formatDuration .Duration}}</td>
                        <td>{{exitCodeBadge .ExitCode}}</td>
                        <td>{{statusBadge .Status}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{else}}
            <div class="empty">No runs yet</div>
            {{end}}
        </div>
    </div>
</body>
</html>`

// jobDetailTemplate is the job detail HTML template
const jobDetailTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; line-height: 1.6; }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        header { background: #2c3e50; color: white; padding: 20px 0; margin-bottom: 30px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        header h1 { font-size: 28px; margin-bottom: 5px; }
        header a { color: white; opacity: 0.8; text-decoration: none; }
        header a:hover { opacity: 1; text-decoration: underline; }
        .job-info { background: white; padding: 25px; border-radius: 8px; margin-bottom: 30px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .job-info h2 { font-size: 20px; margin-bottom: 20px; color: #2c3e50; border-bottom: 2px solid #3498db; padding-bottom: 10px; }
        .info-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 15px; }
        .info-item { padding: 10px 0; }
        .info-item label { display: block; font-size: 12px; color: #7f8c8d; text-transform: uppercase; margin-bottom: 5px; }
        .info-item .value { font-size: 16px; font-weight: 500; }
        .section { background: white; padding: 25px; border-radius: 8px; margin-bottom: 30px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .section h2 { font-size: 20px; margin-bottom: 20px; color: #2c3e50; border-bottom: 2px solid #3498db; padding-bottom: 10px; }
        table { width: 100%; border-collapse: collapse; }
        th { background: #f8f9fa; text-align: left; padding: 12px; font-weight: 600; border-bottom: 2px solid #dee2e6; }
        td { padding: 12px; border-bottom: 1px solid #dee2e6; }
        tr:hover { background: #f8f9fa; }
        .badge { display: inline-block; padding: 4px 8px; border-radius: 4px; font-size: 12px; font-weight: 600; text-transform: uppercase; }
        .badge-success { background: #d4edda; color: #155724; }
        .badge-danger { background: #f8d7da; color: #721c24; }
        .badge-info { background: #d1ecf1; color: #0c5460; }
        .badge-secondary { background: #e2e3e5; color: #383d41; }
        .empty { text-align: center; padding: 40px; color: #7f8c8d; }
        code { background: #f8f9fa; padding: 2px 6px; border-radius: 3px; font-family: monospace; font-size: 13px; }
    </style>
</head>
<body>
    <header>
        <div class="container">
            <div><a href="/">&larr; Back to Dashboard</a></div>
            <h1>{{.Title}}</h1>
        </div>
    </header>

    <div class="container">
        <div class="job-info">
            <h2>Job Details</h2>
            <div class="info-grid">
                <div class="info-item">
                    <label>Job ID</label>
                    <div class="value"><code>{{.Job.ID}}</code></div>
                </div>
                <div class="info-item">
                    <label>Schedule</label>
                    <div class="value"><code>{{.Job.Schedule}}</code></div>
                </div>
                <div class="info-item">
                    <label>Command</label>
                    <div class="value"><code>{{.Job.Command}}</code></div>
                </div>
                <div class="info-item">
                    <label>Last Status</label>
                    <div class="value">{{statusBadge .Job.LastStatus}}</div>
                </div>
                <div class="info-item">
                    <label>Last Run</label>
                    <div class="value">{{formatTime .Job.LastRunTime}}</div>
                </div>
                <div class="info-item">
                    <label>Next Run</label>
                    <div class="value">{{formatTime .Job.NextRunTime}}</div>
                </div>
                <div class="info-item">
                    <label>Success Count</label>
                    <div class="value">{{.Job.SuccessCount}}</div>
                </div>
                <div class="info-item">
                    <label>Failure Count</label>
                    <div class="value">{{.Job.FailureCount}}</div>
                </div>
            </div>
        </div>

        <div class="section">
            <h2>Run History ({{len .Runs}})</h2>
            {{if .Runs}}
            <table>
                <thead>
                    <tr>
                        <th>Run ID</th>
                        <th>Start Time</th>
                        <th>End Time</th>
                        <th>Duration</th>
                        <th>Exit Code</th>
                        <th>Status</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Runs}}
                    <tr>
                        <td><code>{{truncate .RunID 16}}</code></td>
                        <td>{{.StartTime.Format "2006-01-02 15:04:05"}}</td>
                        <td>{{.EndTime.Format "2006-01-02 15:04:05"}}</td>
                        <td>{{formatDuration .Duration}}</td>
                        <td>{{exitCodeBadge .ExitCode}}</td>
                        <td>{{statusBadge .Status}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
            {{else}}
            <div class="empty">No runs yet</div>
            {{end}}
        </div>
    </div>
</body>
</html>`
