package main

import (
	"log"
	"net/http"

	"github.com/antigravity/christmasTournament/internal/db"
	"github.com/antigravity/christmasTournament/internal/handlers"
)

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

func main() {
	db.InitDB("tournament.db")

	// Serve static files
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/static/", noCache(http.StripPrefix("/static/", fs)))

	// API Endpoints
	http.HandleFunc("/api/players", handlers.PlayersHandler)                 // GET, POST
	http.HandleFunc("/api/players/import", handlers.ImportPlayersHandler)    // POST
	http.HandleFunc("/api/players/delete", handlers.DeletePlayerHandler)     // POST
	http.HandleFunc("/api/flights", handlers.FlightsHandler)                 // GET, POST (create)
	http.HandleFunc("/api/flights/update", handlers.UpdateFlightHandler)     // POST
	http.HandleFunc("/api/flights/assign", handlers.AssignPlayerHandler)     // POST (assign)
	http.HandleFunc("/api/flights/unassign", handlers.UnassignPlayerHandler) // POST (unassign)
	http.HandleFunc("/api/scores", handlers.ScoresHandler)                   // POST (submit)
	http.HandleFunc("/api/results", handlers.ResultsHandler)                 // GET
	http.HandleFunc("/api/course", handlers.CourseHandler)                   // GET, POST
	http.HandleFunc("/api/course/import", handlers.ImportCourseHandler)      // POST
	http.HandleFunc("/api/course/export", handlers.ExportCourseHandler)      // GET
	http.HandleFunc("/api/players/fetch-hcp", handlers.FetchHCPHandler)      // POST

	// Admin Pages
	http.HandleFunc("/adminpage", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.ServeFile(w, r, "./web/templates/index.html")
	})

	http.HandleFunc("/adminscorepage", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.ServeFile(w, r, "./web/templates/leaderboard.html")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.ServeFile(w, r, "./web/templates/index.html")
	})

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
