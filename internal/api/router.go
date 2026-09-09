package api

import (
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/minoplhy/nodem/frontend"
	"github.com/minoplhy/nodem/internal/version"
)

// BuildRouter constructs the HTTP router containing all API routes and SPA static file serving.
func BuildRouter(state *AppState) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// API Routes
	r.Route("/api", func(api chi.Router) {
		// Version endpoint
		api.Get("/version", func(w http.ResponseWriter, r *http.Request) {
			RespondJSON(w, http.StatusOK, map[string]string{
				"version":    version.Short(),
				"full":       version.Full(),
				"commit":     version.Commit,
				"build_date": version.BuildDate,
			})
		})

		// Public auth routes
		api.Get("/setup-status", state.SetupStatus)
		api.Post("/setup", state.Setup)
		api.Post("/login", state.Login)
		api.Post("/logout", state.Logout)

		// Agent API routes (authenticated via X-Agent-Token header)
		api.Post("/v1/agent/ech/sync", state.AgentECHSync)
		api.Post("/v1/agent/ech/ack", state.AgentECHAck)

		// Protected routes requiring authentication
		api.Group(func(auth chi.Router) {
			auth.Use(state.RequireAuth)

			auth.Get("/me", state.Me)

			// ECH Clusters & Management
			auth.Get("/ech/clusters", state.ListECHClusters)
			auth.Post("/ech/clusters", state.CreateECHCluster)
			auth.Get("/ech/clusters/{id}", state.GetECHCluster)
			auth.Put("/ech/clusters/{id}", state.UpdateECHCluster)
			auth.Delete("/ech/clusters/{id}", state.DeleteECHCluster)
			auth.Post("/ech/clusters/{id}/rotate", state.TriggerClusterRotation)

			// ECH Nodes & Cluster Associations
			auth.Get("/ech/nodes", state.ListAllECHNodes)
			auth.Post("/ech/nodes", state.CreateIndependentECHNode)
			auth.Get("/ech/nodes/{id}", state.GetECHNodeDetail)
			auth.Put("/ech/nodes/{id}", state.UpdateECHNode)
			auth.Delete("/ech/nodes/{id}", state.DeleteECHNode)
			auth.Put("/ech/nodes/{id}/clusters", state.SetNodeClusters)

			auth.Get("/ech/clusters/{id}/nodes", state.ListECHNodesForCluster)
			auth.Post("/ech/clusters/{id}/nodes", state.CreateECHNode)
			auth.Post("/ech/clusters/{id}/nodes/assign", state.AssignNodeToCluster)
			auth.Delete("/ech/clusters/{id}/nodes/{node_id}", state.UnassignNodeFromCluster)

			auth.Get("/ech/clusters/{id}/domains", state.ListECHDomains)
			auth.Post("/ech/clusters/{id}/domains", state.CreateECHDomain)
			auth.Delete("/ech/domains/{id}", state.DeleteECHDomain)

			auth.Get("/ech/clusters/{id}/logs", state.ListECHLogs)

			// Providers CRUD
			auth.Get("/providers", state.ListProviders)
			auth.Post("/providers", state.CreateProvider)
			auth.Put("/providers/{id}", state.UpdateProvider)
			auth.Delete("/providers/{id}", state.DeleteProvider)

			// Groups
			auth.Get("/groups", state.ListGroups)
			auth.Post("/groups", state.CreateGroup)
			auth.Get("/groups/status", state.GetAllGroupsStatus)
			auth.Get("/groups/{id}", state.GetGroup)
			auth.Put("/groups/{id}", state.UpdateGroup)
			auth.Delete("/groups/{id}", state.DeleteGroup)
			auth.Get("/groups/{id}/config", state.GetGroupConfig)

			// Group operations requiring group access
			auth.Group(func(grp chi.Router) {
				grp.Use(state.RequireGroupAccess)

				grp.Post("/groups/{id}/toggle", state.ToggleGroup)
				grp.Post("/groups/{id}/test", state.TestGroupConfig)
				grp.Get("/groups/{group_id}/status", state.GetGroupStatus)

				// Group IPs
				grp.Get("/groups/{group_id}/ips", state.ListIPs)
				grp.Post("/groups/{group_id}/ips", state.AddIP)
				grp.Put("/groups/{group_id}/ips", state.SyncGroupIPs)
				grp.Delete("/groups/{group_id}/ips/{id}", state.DeleteIP)

				// Group Checks
				grp.Get("/groups/{group_id}/checks", state.ListChecks)
				grp.Post("/groups/{group_id}/checks", state.AddCheck)
				grp.Put("/groups/{group_id}/checks/{id}", state.UpdateCheck)
				grp.Delete("/groups/{group_id}/checks/{id}", state.DeleteCheck)

				// Group Rules
				grp.Get("/groups/{group_id}/rules", state.ListRules)
				grp.Post("/groups/{group_id}/rules", state.AddRule)
				grp.Delete("/groups/{group_id}/rules/{id}", state.DeleteRule)

				// Group Notifications linkage
				grp.Get("/groups/{group_id}/notifications", state.ListGroupNotifications)
				grp.Post("/groups/{group_id}/notifications", state.LinkGroupNotification)
				grp.Delete("/groups/{group_id}/notifications/{channel_id}", state.UnlinkGroupNotification)

				// Audit Logs
				grp.Get("/groups/{group_id}/logs", state.GetGroupLogs)
			})

			// Notification Channels CRUD
			auth.Get("/notifications", state.ListNotificationChannels)
			auth.Post("/notifications", state.CreateNotificationChannel)
			auth.Put("/notifications/{id}", state.UpdateNotificationChannel)
			auth.Delete("/notifications/{id}", state.DeleteNotificationChannel)
			auth.Post("/notifications/{id}/test", state.TestNotificationChannel)

			// Sessions management
			auth.Get("/sessions", state.ListActiveSessions)
			auth.Delete("/sessions/{id}", state.RevokeSession)
		})
	})

	// Static files & SPA fallback
	frontendDir := ""
	candidates := []string{
		"frontend/dist",
		"../frontend/dist",
		"../../frontend/dist",
		"../node_monitor/frontend/dist",
		"../../node_monitor/frontend/dist",
	}
	if cwd, err := os.Getwd(); err == nil {
		for _, cand := range candidates {
			p := filepath.Join(cwd, cand)
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				frontendDir = p
				break
			}
		}
	}
	if frontendDir == "" {
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			for _, cand := range candidates {
				p := filepath.Join(exeDir, cand)
				if info, err := os.Stat(p); err == nil && info.IsDir() {
					frontendDir = p
					break
				}
			}
		}
	}
	if frontendDir == "" {
		frontendDir = "frontend/dist"
	}

	assetsDir := filepath.Join(frontendDir, "assets")

	r.Handle("/assets/*", serveAssetHandler(assetsDir))
	r.NotFound(serveIndexHandler(state, frontendDir))

	// Handle custom BASE_PATH if configured
	if state.BasePath != "" && state.BasePath != "/" {
		nestedPath := "/" + strings.Trim(state.BasePath, "/")
		nestedPathSlash := nestedPath + "/"

		outer := chi.NewRouter()
		outer.Use(middleware.RequestID)
		outer.Use(middleware.RealIP)
		outer.Use(middleware.Recoverer)

		// 1. Stateless redirect: /monitor -> /monitor/
		redirectMonitor := func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, nestedPathSlash, http.StatusPermanentRedirect)
		}
		outer.Get(nestedPath, redirectMonitor)
		outer.Head(nestedPath, redirectMonitor)

		// 2. Mount inner router under /monitor/
		outer.Mount(nestedPathSlash, r)

		// 3. Mount /assets/* directly on outer router as well, ensuring that any asset
		// requests made without base prefix (or stripped by reverse proxies) are served
		// with proper MIME types instead of returning 404 text/plain.
		outer.Handle("/assets/*", serveAssetHandler(assetsDir))

		// 4. Static icons and favicons on outer router
		outer.Get("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/svg+xml")
			if _, err := os.Stat(filepath.Join(frontendDir, "favicon.svg")); err == nil {
				http.ServeFile(w, r, filepath.Join(frontendDir, "favicon.svg"))
				return
			}
			if frontend.HasEmbedded() {
				if distFS, err := frontend.Dist(); err == nil {
					if data, err := fs.ReadFile(distFS, "favicon.svg"); err == nil {
						_, _ = w.Write(data)
						return
					}
				}
			}
			http.NotFound(w, r)
		})
		outer.Get("/icons.svg", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/svg+xml")
			if _, err := os.Stat(filepath.Join(frontendDir, "icons.svg")); err == nil {
				http.ServeFile(w, r, filepath.Join(frontendDir, "icons.svg"))
				return
			}
			if frontend.HasEmbedded() {
				if distFS, err := frontend.Dist(); err == nil {
					if data, err := fs.ReadFile(distFS, "icons.svg"); err == nil {
						_, _ = w.Write(data)
						return
					}
				}
			}
			http.NotFound(w, r)
		})

		// 5. Redirect root / -> /monitor/
		redirectRoot := func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, nestedPathSlash, http.StatusTemporaryRedirect)
		}
		outer.Get("/", redirectRoot)
		outer.Head("/", redirectRoot)

		// 6. Any other unmatched root requests fall through to index handler
		outer.NotFound(serveIndexHandler(state, frontendDir))

		return outer
	}

	return r
}

func init() {
	// Explicitly register web MIME types to ensure correct Content-Type headers
	// regardless of host operating system or missing /etc/mime.types in containers.
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".mjs", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
	_ = mime.AddExtensionType(".json", "application/json; charset=utf-8")
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".ico", "image/x-icon")
	_ = mime.AddExtensionType(".png", "image/png")
	_ = mime.AddExtensionType(".jpg", "image/jpeg")
	_ = mime.AddExtensionType(".jpeg", "image/jpeg")
	_ = mime.AddExtensionType(".webp", "image/webp")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
	_ = mime.AddExtensionType(".woff", "font/woff")
	_ = mime.AddExtensionType(".ttf", "font/ttf")
}

func serveAssetHandler(assetsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := chi.URLParam(r, "*")
		if file == "" {
			if idx := strings.Index(r.URL.Path, "/assets/"); idx != -1 {
				file = r.URL.Path[idx+len("/assets/"):]
			}
		}

		cleanFile := filepath.Clean("/" + strings.TrimPrefix(file, "/"))
		if cleanFile == "/" || cleanFile == "." {
			http.NotFound(w, r)
			return
		}

		ext := strings.ToLower(filepath.Ext(cleanFile))
		ctype := mime.TypeByExtension(ext)
		if ctype == "" {
			switch ext {
			case ".js", ".mjs":
				ctype = "text/javascript; charset=utf-8"
			case ".css":
				ctype = "text/css; charset=utf-8"
			case ".svg":
				ctype = "image/svg+xml"
			case ".json":
				ctype = "application/json; charset=utf-8"
			case ".wasm":
				ctype = "application/wasm"
			case ".ico":
				ctype = "image/x-icon"
			case ".png":
				ctype = "image/png"
			default:
				ctype = "application/octet-stream"
			}
		}

		// 1. Try local disk
		filePath := filepath.Join(assetsDir, cleanFile)
		cleanAssetsDir := filepath.Clean(assetsDir)
		if strings.HasPrefix(filepath.Clean(filePath), cleanAssetsDir) {
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				w.Header().Set("Content-Type", ctype)
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				http.ServeFile(w, r, filePath)
				return
			}
		}

		// 2. Try embedded frontend
		if frontend.HasEmbedded() {
			if distFS, err := frontend.Dist(); err == nil {
				assetPath := strings.TrimPrefix(filepath.Clean("assets/"+cleanFile), "/")
				if data, err := fs.ReadFile(distFS, assetPath); err == nil {
					w.Header().Set("Content-Type", ctype)
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					_, _ = w.Write(data)
					return
				}
			}
		}

		http.NotFound(w, r)
	}
}

func serveIndexHandler(state *AppState, frontendDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// API routes that reach not-found handler should return JSON 404, not HTML
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.Contains(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Not Found"}`))
			return
		}

		// Check if a static file in frontendDir is being requested (e.g. /favicon.svg, /icons.svg)
		reqPath := r.URL.Path
		if state.BasePath != "" && state.BasePath != "/" {
			basePrefix := "/" + strings.Trim(state.BasePath, "/")
			reqPath = strings.TrimPrefix(reqPath, basePrefix)
		}
		cleanReqPath := filepath.Clean("/" + strings.TrimPrefix(reqPath, "/"))

		if cleanReqPath != "/" && cleanReqPath != "." {
			possibleFile := filepath.Join(frontendDir, cleanReqPath)
			cleanFrontendDir := filepath.Clean(frontendDir)
			if strings.HasPrefix(filepath.Clean(possibleFile), cleanFrontendDir) {
				if info, err := os.Stat(possibleFile); err == nil && !info.IsDir() {
					ext := strings.ToLower(filepath.Ext(possibleFile))
					ctype := mime.TypeByExtension(ext)
					if ctype != "" {
						w.Header().Set("Content-Type", ctype)
					}
					http.ServeFile(w, r, possibleFile)
					return
				}
			}

			// Try embedded fallback for static files
			if frontend.HasEmbedded() {
				if distFS, err := frontend.Dist(); err == nil {
					relPath := strings.TrimPrefix(cleanReqPath, "/")
					if data, err := fs.ReadFile(distFS, relPath); err == nil {
						ext := strings.ToLower(filepath.Ext(relPath))
						ctype := mime.TypeByExtension(ext)
						if ctype != "" {
							w.Header().Set("Content-Type", ctype)
						}
						_, _ = w.Write(data)
						return
					}
				}
			}
		}

		var htmlBytes []byte
		var err error

		if info, statErr := os.Stat(filepath.Join(frontendDir, "index.html")); statErr == nil && !info.IsDir() {
			htmlBytes, err = os.ReadFile(filepath.Join(frontendDir, "index.html"))
		}

		if (err != nil || len(htmlBytes) == 0) && frontend.HasEmbedded() {
			if distFS, subErr := frontend.Dist(); subErr == nil {
				htmlBytes, err = fs.ReadFile(distFS, "index.html")
			}
		}

		if err != nil || len(htmlBytes) == 0 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintf(w, "<html><body>index.html not found</body></html>")
			return
		}

		basePath := "/"
		if state.BasePath != "" && state.BasePath != "/" {
			basePath = "/" + strings.Trim(state.BasePath, "/") + "/"
		}

		injectedHTML := string(htmlBytes)

		// 1. Inject <base href="..."> and <script>window.__BASE_PATH__ = "...";</script> into <head>
		headTags := fmt.Sprintf("<base href=\"%s\">\n    <script>window.__BASE_PATH__ = \"%s\";</script>", basePath, basePath)
		injectedHTML = strings.Replace(injectedHTML, "<head>", "<head>\n    "+headTags, 1)

		// 2. Rewrite relative asset paths to absolute basePath so browser loads them reliably
		injectedHTML = strings.ReplaceAll(injectedHTML, "src=\"./assets/", fmt.Sprintf("src=\"%sassets/", basePath))
		injectedHTML = strings.ReplaceAll(injectedHTML, "href=\"./assets/", fmt.Sprintf("href=\"%sassets/", basePath))
		injectedHTML = strings.ReplaceAll(injectedHTML, "src=\"assets/", fmt.Sprintf("src=\"%sassets/", basePath))
		injectedHTML = strings.ReplaceAll(injectedHTML, "href=\"assets/", fmt.Sprintf("href=\"%sassets/", basePath))

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(injectedHTML))
	}
}
