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
	case "environment-profiles":
		if method == http.MethodGet && collection {
			return "environments.read"
		}
	case "environment-sources":
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
	case "environment-releases":
		if method == http.MethodGet && (collection || detail || action == "manifest" || action == "quality" || action == "stac") {
			return "environments.read"
		}
		if method == http.MethodGet && (action == "prepared" || action == "rebuild") {
			return "environments.download"
		}
		if method == http.MethodPost && action == "verify" {
			return "environments.qualify"
		}
		if method == http.MethodPost && (action == "release" || action == "withdraw") {
			return "environments.release"
		}
	}
	return ""
}
