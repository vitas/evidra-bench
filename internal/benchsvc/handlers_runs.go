package benchsvc

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/vitas/evidra-bench/internal/apiutil"
	"github.com/vitas/evidra-bench/internal/auth"
	"github.com/vitas/evidra-bench/pkg/artifact"
	bench "github.com/vitas/evidra-bench/pkg/bench"
	"github.com/vitas/evidra-bench/pkg/runreview"
)

func handleListRuns(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		f := runFiltersFromRequest(r)

		runs, total, err := svc.ListRuns(r.Context(), tenantID, f)
		if err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if runs == nil {
			runs = []bench.RunRecord{}
		}
		if err := attachReviewSummaries(r, svc, tenantID, runs); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, map[string]any{
			"runs":   runs,
			"total":  total,
			"limit":  f.Limit,
			"offset": f.Offset,
		})
	}
}

func runFiltersFromRequest(r *http.Request) bench.RunFilters {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(q.Get("offset"))

	f := bench.RunFilters{
		ScenarioID:              q.Get("scenario"),
		ScenarioIDs:             parseCSVQuery(q.Get("scenarios")),
		Model:                   q.Get("model"),
		Provider:                q.Get("provider"),
		ToolServer:              q.Get("tool_server"),
		ToolServerVersion:       q.Get("tool_server_version"),
		ReportID:                q.Get("report_id"),
		SkillID:                 q.Get("skill_id"),
		SkillVersion:            q.Get("skill_version"),
		SkillUnset:              q.Get("skill_unset") == "true",
		ToolServerUnset:         q.Get("tool_server_unset") == "true",
		Since:                   parseSince(q.Get("since")),
		Limit:                   limit,
		Offset:                  offset,
		SortBy:                  q.Get("sort_by"),
		SortOrder:               q.Get("sort_order"),
		ReviewState:             q.Get("review"),
		ReviewVerdict:           q.Get("review_verdict"),
		ReviewSeverity:          q.Get("review_severity"),
		ReviewVisibility:        q.Get("review_visibility"),
		Reviewer:                q.Get("reviewer"),
		ReviewHasSuggestedRules: q.Get("has_suggested_rules") == "true",
		ReviewIncludePrivate:    !isAnonymousRead(r),
	}
	if q.Get("passed") == "true" {
		f.PassedOnly = true
	}
	if q.Get("passed") == "false" {
		f.FailedOnly = true
	}
	if q.Get("exclude_errors") == "true" {
		f.ExcludeErrors = true
	}
	return f
}

func handleGetRun(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := auth.TenantID(r.Context())
		id := r.PathValue("id")
		run, err := svc.GetRun(r.Context(), tenantID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				apiutil.WriteError(w, http.StatusNotFound, "run not found")
			} else {
				apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
		if err := attachReviewSummary(r, svc, tenantID, run); err != nil {
			apiutil.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		apiutil.WriteJSON(w, http.StatusOK, run)
	}
}

func attachReviewSummaries(r *http.Request, svc *Service, tenantID string, runs []bench.RunRecord) error {
	for i := range runs {
		if err := attachReviewSummary(r, svc, tenantID, &runs[i]); err != nil {
			return err
		}
	}
	return nil
}

func attachReviewSummary(r *http.Request, svc *Service, tenantID string, run *bench.RunRecord) error {
	if run == nil || strings.TrimSpace(run.ID) == "" {
		return nil
	}
	data, _, err := svc.GetArtifact(r.Context(), tenantID, run.ID, artifact.HostedRunReview)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil
	}
	review, err := runreview.Decode(data)
	if err != nil {
		return err
	}
	if isAnonymousRead(r) && !runreview.IsPublic(review) {
		return nil
	}
	summary := summarizeRunReview(review)
	run.ReviewSummary = &summary
	return nil
}

func isAnonymousRead(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("Authorization")) == "" && !hasSessionCookie(r)
}

func summarizeRunReview(review runreview.Review) bench.RunReviewSummary {
	primaryLabel := review.PrimaryLabel
	if primaryLabel == "" && len(review.Labels) > 0 {
		primaryLabel = review.Labels[0].Kind
	}
	return bench.RunReviewSummary{
		Verdict:                review.Verdict,
		PrimaryLabel:           primaryLabel,
		Visibility:             review.Visibility,
		LabelCount:             len(review.Labels),
		MaxSeverity:            maxReviewSeverity(review.Labels),
		SuggestedRuleCount:     len(review.SuggestedRules),
		PrimaryEvidenceSnippet: primaryEvidenceSnippet(review, primaryLabel),
	}
}

func primaryEvidenceSnippet(review runreview.Review, primaryLabel string) string {
	if len(review.Labels) == 0 {
		return ""
	}
	if primaryLabel != "" {
		for _, label := range review.Labels {
			if label.Kind == primaryLabel && strings.TrimSpace(label.EvidenceSnippet) != "" {
				return label.EvidenceSnippet
			}
		}
	}
	for _, label := range review.Labels {
		if strings.TrimSpace(label.EvidenceSnippet) != "" {
			return label.EvidenceSnippet
		}
	}
	return ""
}

func primaryReviewerNote(review runreview.Review, primaryLabel string) string {
	if len(review.Labels) == 0 {
		return ""
	}
	if primaryLabel != "" {
		for _, label := range review.Labels {
			if label.Kind == primaryLabel && strings.TrimSpace(label.Note) != "" {
				return label.Note
			}
		}
	}
	for _, label := range review.Labels {
		if strings.TrimSpace(label.Note) != "" {
			return label.Note
		}
	}
	return ""
}

func maxReviewSeverity(labels []runreview.Label) string {
	maxSeverity := ""
	maxRank := -1
	for _, label := range labels {
		severity := label.Severity
		if severity == "" {
			severity = runreview.SeverityInfo
		}
		if rank := reviewSeverityRank(severity); rank > maxRank {
			maxRank = rank
			maxSeverity = severity
		}
	}
	return maxSeverity
}

func reviewSeverityRank(severity string) int {
	switch severity {
	case runreview.SeverityCritical:
		return 4
	case runreview.SeverityError:
		return 3
	case runreview.SeverityWarning:
		return 2
	case runreview.SeverityInfo:
		return 1
	default:
		return 0
	}
}
