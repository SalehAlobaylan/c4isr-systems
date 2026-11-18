package httpx

import "net/http"

func geographicEnvironmentPermission(method string, parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	collection, detail := len(parts) == 1, len(parts) == 2
	action := ""
	if len(parts) == 3 {
		action = parts[2]
	}
	switch parts[0] {
	case "environment-operations":
		if method == http.MethodGet && detail && parts[1] == "metrics" {
			return "operational.read"
		}
		if method == http.MethodGet && detail && parts[1] == "storage" {
			return "environments.manage"
		}
	case "environment-worker":
		if method == http.MethodPost && detail && (parts[1] == "claim" || parts[1] == "reconcile") {
			return "environments.execute"
		}
		if len(parts) == 4 && parts[1] == "builds" && method == http.MethodPost &&
			(parts[3] == "heartbeat" || parts[3] == "fail" || parts[3] == "complete") {
			return "environments.execute"
		}
		if len(parts) == 5 && parts[1] == "builds" && parts[3] == "inputs" && method == http.MethodGet {
			return "environments.execute"
		}
	case "environment-profiles":
		if method == http.MethodGet && collection {
			return "environments.read"
		}
	case "environment-sources":
		if method == http.MethodGet && action == "impact" {
			return "environments.manage"
		}
		if method == http.MethodPost && action == "admissions" {
			return "environments.manage"
		}
	case "geographic-environments":
		if method == http.MethodGet && (collection || detail || action == "preflight" || action == "eligibility") {
			return "environments.read"
		}
		if method == http.MethodPost && collection || method == http.MethodPut && detail {
			return "environments.manage"
		}
		if method == http.MethodPost && action == "builds" {
			return "environments.build"
		}
	case "environment-assets":
		if method == http.MethodGet && collection {
			return "environments.read"
		}
		if method == http.MethodPost && collection || method == http.MethodPut && action == "content" {
			return "environments.manage"
		}
		if method == http.MethodGet && action == "content" {
			return "environments.download"
		}
	case "environment-builds":
		if method == http.MethodGet && (detail || action == "attempts") {
			return "environments.read"
		}
		if method == http.MethodPost && (action == "cancel" || action == "retry") {
			return "environments.build"
		}
	case "environment-display-builds":
		if method == http.MethodPost && action == "cancel" {
			return "environments.qualify"
		}
	case "environment-releases":
		if method == http.MethodGet && (collection || detail || action == "manifest" || action == "lineage" || action == "quality" || action == "quality-report" || action == "stac" || action == "displays" || action == "decisions" || action == "assessments") {
			return "environments.read"
		}
		if method == http.MethodGet && (action == "prepared" || action == "rebuild") {
			return "environments.download"
		}
		if method == http.MethodPost && action == "viewing-packages" {
			return "environments.download"
		}
		if method == http.MethodPost && (action == "verify" || action == "displays" || action == "display-builds" || action == "assessments") {
			return "environments.qualify"
		}
		if method == http.MethodPost && action == "suitability" {
			return "environments.read"
		}
		if method == http.MethodPost && (action == "release" || action == "withdraw") {
			return "environments.release"
		}
	case "environment-assessments":
		if method == http.MethodGet && detail {
			return "environments.read"
		}
		if method == http.MethodPost && action == "supersede" {
			return "environments.qualify"
		}
	case "environment-displays":
		if method == http.MethodGet && action == "manifest" {
			return "environments.read"
		}
		if method == http.MethodGet && action == "content" {
			return "environments.download"
		}
		if method == http.MethodPut && action == "content" {
			return "environments.qualify"
		}
	}
	return ""
}
