package httpx

import "net/http"

func simPlatformPermission(method string, parts []string) string {
	if len(parts) < 2 || parts[0] != "sim-platform" {
		return ""
	}
	sub := parts[1]
	switch sub {
	case "health":
		if method == http.MethodGet && len(parts) == 2 {
			return "operational.read"
		}
	case "biomes":
		if method == http.MethodGet && (len(parts) == 2 || (len(parts) == 4 && (parts[3] == "terrain.glb" || parts[3] == "imagery.json"))) {
			return "operational.read"
		}
	case "networks":
		if (method == http.MethodGet || method == http.MethodPost) && len(parts) == 2 {
			return "operational.read"
		}
	case "runs":
		if method == http.MethodGet && (len(parts) == 2 || len(parts) == 3 || (len(parts) == 4 && (parts[3] == "events" || parts[3] == "replay" || parts[3] == "replay.json"))) {
			return "operational.read"
		}
		if method == http.MethodPost && len(parts) == 4 && parts[3] == "cancel" {
			return "operational.read"
		}
	case "sim", "coverage", "salvo":
		if len(parts) == 3 {
			switch parts[2] {
			case "health", "environments", "networks":
				if method == http.MethodGet {
					return "operational.read"
				}
			case "runs":
				if method == http.MethodGet || method == http.MethodPost {
					return "operational.read"
				}
			case "los-profile", "preview-sensor", "adversarial-route", "export-network":
				if sub == "coverage" && method == http.MethodPost {
					return "operational.read"
				}
			}
		}
		if len(parts) == 4 && parts[2] == "runs" && method == http.MethodGet {
			return "operational.read"
		}
		if len(parts) == 5 && parts[2] == "runs" {
			if method == http.MethodGet && (parts[4] == "events" || parts[4] == "replay" || parts[4] == "replay.json" || parts[4] == "terrain.glb" || parts[4] == "imagery.json") {
				return "operational.read"
			}
			if method == http.MethodPost && parts[4] == "cancel" {
				return "operational.read"
			}
		}
	case "bench":
		if len(parts) == 3 {
			switch parts[2] {
			case "health", "environments", "models", "leaderboard":
				if method == http.MethodGet {
					return "operational.read"
				}
			case "monte-carlo", "benchmark":
				if method == http.MethodPost {
					return "operational.read"
				}
			}
		}
	case "sessions":
		if len(parts) == 2 && (method == http.MethodGet || method == http.MethodPost) {
			return "operational.read"
		}
		if len(parts) == 3 && method == http.MethodGet {
			return "operational.read"
		}
		if len(parts) == 4 {
			if method == http.MethodGet && parts[3] == "events" {
				return "operational.read"
			}
			if method == http.MethodPost && (parts[3] == "commands" || parts[3] == "control" || parts[3] == "step" || parts[3] == "stop") {
				return "operational.read"
			}
		}
	}
	return ""
}
