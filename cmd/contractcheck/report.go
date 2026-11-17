package main

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mask replaces a secret in every line the report prints.
const mask = "***"

// minSecret is the shortest string the redactor masks, so that a one-letter
// password does not blank the report.
const minSecret = 4

// result is the outcome of one call of one route in one scenario.
type result struct {
	Route    string
	Method   string
	Path     string
	Scenario string
	// Status is the HTTP status; 0 when the call was skipped.
	Status int
	// Bytes is the size of the body.
	Bytes int
	// Skip is why the call did not run; empty when it did.
	Skip string
	// Findings are the differences found.
	Findings []finding
}

// verdict returns "PASS", "WARN", "FAIL" or "SKIP".
func (r result) verdict() string {
	if r.Skip != "" {
		return "SKIP"
	}
	worst := sevUnknown
	for _, f := range r.Findings {
		worst = max(worst, f.Sev)
	}
	switch worst {
	case sevError:
		return "FAIL"
	case sevWarning:
		return "WARN"
	case sevUnknown:
		return "PASS"
	default:
		return "PASS"
	}
}

// report collects the results of a run and prints them. It is used from one
// goroutine.
type report struct {
	results []result
	secrets []string
	logf    func(format string, args ...any)
}

// summary counts a report.
type summary struct {
	// Routes is the number of distinct routes that ran.
	Routes int
	// Checks is the number of calls that ran.
	Checks   int
	Errors   int
	Warnings int
	Skipped  int
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newReport returns an empty report; logf, when not nil, gets one line per
// call as it is recorded (-v).
func newReport(logf func(format string, args ...any)) *report {
	return &report{logf: logf}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// addSecret registers a string to mask in the output.
func (r *report) addSecret(s string) {
	if len(s) >= minSecret && !slices.Contains(r.secrets, s) {
		r.secrets = append(r.secrets, s)
	}
}

// redact masks every registered secret in s.
func (r *report) redact(s string) string {
	for _, sec := range r.secrets {
		s = strings.ReplaceAll(s, sec, mask)
	}

	return s
}

// add records a result.
func (r *report) add(res result) {
	r.results = append(r.results, res)
	if r.logf != nil {
		r.logf("%-4s %-16s %-18s %3d %6d B %s", res.verdict(), res.Route, res.Scenario, res.Status, res.Bytes, r.redact(res.Skip))
	}
}

// summary counts the results.
func (r *report) summary() summary {
	var s summary
	routes := map[string]bool{}
	for _, res := range r.results {
		if res.Skip != "" {
			s.Skipped++

			continue
		}
		s.Checks++
		routes[res.Route] = true
		for _, f := range res.Findings {
			switch f.Sev {
			case sevError:
				s.Errors++
			case sevWarning:
				s.Warnings++
			case sevUnknown:
			default:
			}
		}
	}
	s.Routes = len(routes)

	return s
}

// passed reports whether the run has no error.
func (r *report) passed() bool { return r.summary().Errors == 0 }

// write prints the table, the findings and the summary line. mode is "self"
// or the scheme and host of the server.
func (r *report) write(w io.Writer, mode string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ROUTE\tREQUEST\tSCENARIO\tSTATUS\tRESULT\tNOTE")
	for _, res := range r.results {
		status := "-"
		if res.Status != 0 {
			status = strconv.Itoa(res.Status)
		}
		note := r.redact(res.Skip)
		if note == "" {
			note = r.redact(findingsNote(res.Findings))
		}
		fmt.Fprintf(tw, "%s\t%s %s\t%s\t%s\t%s\t%s\n", res.Route, res.Method, res.Path, res.Scenario, status, res.verdict(), note)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	var details []string
	for _, res := range r.results {
		for _, f := range res.Findings {
			details = append(details, fmt.Sprintf("%-7s %s [%s] %s: %s", f.Sev, res.Route, res.Scenario, f.Path, r.redact(f.Msg)))
		}
	}
	if len(details) > 0 {
		fmt.Fprintln(w)
		for _, d := range details {
			fmt.Fprintln(w, d)
		}
	}

	s := r.summary()
	word := "PASS"
	if s.Errors > 0 {
		word = "FAIL"
	}
	fmt.Fprintf(w, "\ncontractcheck %s: %s, %d routes, %d checks, %d errors, %d warnings, %d skipped\n",
		mode, word, s.Routes, s.Checks, s.Errors, s.Warnings, s.Skipped)

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// findingsNote is the short note of a table row: the first finding and how
// many more.
func findingsNote(fs []finding) string {
	if len(fs) == 0 {
		return ""
	}
	note := fs[0].Path + " " + fs[0].Msg
	if len(fs) > 1 {
		note += " (+" + strconv.Itoa(len(fs)-1) + " more)"
	}

	return note
}
