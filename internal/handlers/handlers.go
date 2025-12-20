package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/antigravity/christmasTournament/internal/db"
	"github.com/antigravity/christmasTournament/internal/models"
)

func PlayersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := db.DB.Query("SELECT id, name, surname, reg_num, handicap, gender FROM players")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var players []models.Player
		for rows.Next() {
			var p models.Player
			if err := rows.Scan(&p.ID, &p.Name, &p.Surname, &p.RegNum, &p.Handicap, &p.Gender); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			players = append(players, p)
		}
		json.NewEncoder(w).Encode(players)
	} else if r.Method == http.MethodPost {
		var p models.Player
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if p.ID > 0 {
			// Update
			_, err := db.DB.Exec("UPDATE players SET name=?, surname=?, reg_num=?, handicap=?, gender=? WHERE id=?", p.Name, p.Surname, p.RegNum, p.Handicap, p.Gender, p.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		} else {
			// Create
			_, err := db.DB.Exec("INSERT INTO players (name, surname, reg_num, handicap, gender) VALUES (?, ?, ?, ?, ?)", p.Name, p.Surname, p.RegNum, p.Handicap, p.Gender)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
		}
	}
}

func DeletePlayerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Expecting JSON body with ID
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := db.DB.Exec("DELETE FROM players WHERE id = ?", req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func ImportPlayersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Optional: Skip header if present. For now, assume no header or user handles it.
	// But usually CSVs have headers. Let's try to peek or just skip first line if it contains "Name"

	records, err := reader.ReadAll()
	if err != nil {
		http.Error(w, "Failed to parse CSV", http.StatusBadRequest)
		return
	}

	for i, record := range records {
		// Skip header if it looks like one
		if i == 0 && (record[0] == "Name" || record[0] == "name") {
			continue
		}
		if len(record) < 3 {
			continue // Skip invalid rows
		}

		name := record[0]
		surname := record[1]
		regNum := record[2]
		var handicap float64
		if len(record) > 3 {
			if h, err := strconv.ParseFloat(record[3], 64); err == nil {
				handicap = h
			}
		}

		_, err = db.DB.Exec("INSERT INTO players (name, surname, reg_num, handicap) VALUES (?, ?, ?, ?)", name, surname, regNum, handicap)
		if err != nil {
			// Log error but continue? Or fail?
			// Let's continue
			continue
		}
	}

	w.WriteHeader(http.StatusOK)
}

func FlightsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Get all flights with players
		rows, err := db.DB.Query(`
			SELECT f.id, f.token, f.name, f.starting_hole, p.id, p.name, p.surname, p.reg_num, p.handicap, p.gender
			FROM flights f
			LEFT JOIN flight_players fp ON f.id = fp.flight_id
			LEFT JOIN players p ON fp.player_id = p.id
		`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		flightMap := make(map[int]*struct {
			ID           int             `json:"id"`
			Token        string          `json:"token"`
			Name         string          `json:"name"`
			StartingHole int             `json:"starting_hole"`
			Players      []models.Player `json:"players"`
		})

		for rows.Next() {
			var fID, fStartingHole int
			var fToken, fName string
			var pID sql.NullInt64
			var pName, pSurname, pRegNum, pGender sql.NullString
			var pHandicap sql.NullFloat64

			if err := rows.Scan(&fID, &fToken, &fName, &fStartingHole, &pID, &pName, &pSurname, &pRegNum, &pHandicap, &pGender); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if _, ok := flightMap[fID]; !ok {
				flightMap[fID] = &struct {
					ID           int             `json:"id"`
					Token        string          `json:"token"`
					Name         string          `json:"name"`
					StartingHole int             `json:"starting_hole"`
					Players      []models.Player `json:"players"`
				}{
					ID:           fID,
					Token:        fToken,
					Name:         fName,
					StartingHole: fStartingHole,
					Players:      []models.Player{},
				}
			}

			if pID.Valid {
				flightMap[fID].Players = append(flightMap[fID].Players, models.Player{
					ID:       int(pID.Int64),
					Name:     pName.String,
					Surname:  pSurname.String,
					RegNum:   pRegNum.String,
					Handicap: pHandicap.Float64,
					Gender:   pGender.String,
				})
			}
		}

		var keys []int
		for k := range flightMap {
			keys = append(keys, k)
		}
		sort.Ints(keys)

		var flights []interface{}
		for _, k := range keys {
			flights = append(flights, flightMap[k])
		}
		json.NewEncoder(w).Encode(flights)

	} else if r.Method == http.MethodPost {
		// Create new flight
		var req struct {
			Name         string `json:"name"`
			StartingHole int    `json:"starting_hole"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.StartingHole == 0 {
			req.StartingHole = 1
		}

		// Generate a hash token
		hash := sha256.Sum256([]byte(req.Name + time.Now().String()))
		token := hex.EncodeToString(hash[:])[:16] // Take first 16 chars

		res, err := db.DB.Exec("INSERT INTO flights (token, name, starting_hole) VALUES (?, ?, ?)", token, req.Name, req.StartingHole)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "token": token, "starting_hole": req.StartingHole})
	} else if r.Method == http.MethodDelete {
		// Delete flight
		var req struct {
			ID int `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Transaction
		tx, err := db.DB.Begin()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Remove players from flight
		_, err = tx.Exec("DELETE FROM flight_players WHERE flight_id = ?", req.ID)
		if err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Delete flight
		_, err = tx.Exec("DELETE FROM flights WHERE id = ?", req.ID)
		if err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func UpdateFlightHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		StartingHole int    `json:"starting_hole"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := db.DB.Exec("UPDATE flights SET name = ?, starting_hole = ? WHERE id = ?", req.Name, req.StartingHole, req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func AssignPlayerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FlightID int `json:"flight_id"`
		PlayerID int `json:"player_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if flight is full (max 4)
	var count int
	err := db.DB.QueryRow("SELECT COUNT(*) FROM flight_players WHERE flight_id = ?", req.FlightID).Scan(&count)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if count >= 4 {
		http.Error(w, "Flight is full", http.StatusBadRequest)
		return
	}

	// Transaction to ensure atomicity
	tx, err := db.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Remove from any existing flight
	_, err = tx.Exec("DELETE FROM flight_players WHERE player_id = ?", req.PlayerID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add to new flight
	_, err = tx.Exec("INSERT INTO flight_players (flight_id, player_id) VALUES (?, ?)", req.FlightID, req.PlayerID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func UnassignPlayerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PlayerID int `json:"player_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := db.DB.Exec("DELETE FROM flight_players WHERE player_id = ?", req.PlayerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func ScoresHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		playerIDStr := r.URL.Query().Get("player_id")
		if playerIDStr == "" {
			http.Error(w, "Missing player_id", http.StatusBadRequest)
			return
		}
		playerID, _ := strconv.Atoi(playerIDStr)

		rows, err := db.DB.Query("SELECT hole_number, strokes FROM scores WHERE player_id = ?", playerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		scores := make(map[int]int)
		for rows.Next() {
			var hole, strokes int
			if err := rows.Scan(&hole, &strokes); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			scores[hole] = strokes
		}
		json.NewEncoder(w).Encode(scores)

	} else if r.Method == http.MethodPost {
		var s models.Score
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Max score 11
		if s.Strokes > 11 {
			s.Strokes = 11
		}

		// Upsert score (delete existing for this hole/player then insert, or use ON CONFLICT if supported/configured)
		// Simple way: check if exists
		var exists int
		db.DB.QueryRow("SELECT id FROM scores WHERE player_id = ? AND hole_number = ?", s.PlayerID, s.HoleNumber).Scan(&exists)

		if exists > 0 {
			_, err := db.DB.Exec("UPDATE scores SET strokes = ? WHERE id = ?", s.Strokes, exists)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			_, err := db.DB.Exec("INSERT INTO scores (player_id, hole_number, strokes) VALUES (?, ?, ?)", s.PlayerID, s.HoleNumber, s.Strokes)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

func ResultsHandler(w http.ResponseWriter, r *http.Request) {
	// Calculate total scores
	rows, err := db.DB.Query(`
		SELECT p.id, p.name, p.surname, p.handicap, COALESCE(SUM(s.strokes), 0) as total_strokes, COUNT(s.id) as holes_played
		FROM players p
		LEFT JOIN scores s ON p.id = s.player_id
		GROUP BY p.id
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var pID int
		var pName, pSurname string
		var pHandicap float64
		var totalStrokes, holesPlayed int
		if err := rows.Scan(&pID, &pName, &pSurname, &pHandicap, &totalStrokes, &holesPlayed); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		netScore := float64(totalStrokes) - pHandicap
		results = append(results, map[string]interface{}{
			"id":           pID,
			"name":         pName,
			"surname":      pSurname,
			"handicap":     pHandicap,
			"gross":        totalStrokes,
			"net":          netScore,
			"holes_played": holesPlayed,
		})
	}

	// Sort by Net score
	sort.Slice(results, func(i, j int) bool {
		// If gross is 0, they haven't started or have 0 strokes.
		// Put players with 0 holes at the bottom
		if results[i]["holes_played"].(int) == 0 && results[j]["holes_played"].(int) > 0 {
			return false
		}
		if results[j]["holes_played"].(int) == 0 && results[i]["holes_played"].(int) > 0 {
			return true
		}
		return results[i]["net"].(float64) < results[j]["net"].(float64)
	})

	json.NewEncoder(w).Encode(results)
}

func CourseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := db.DB.Query("SELECT hole_number, par, length_yellow, length_red FROM holes ORDER BY hole_number")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type HoleInfo struct {
			HoleNumber   int `json:"hole_number"`
			Par          int `json:"par"`
			LengthYellow int `json:"length_yellow"`
			LengthRed    int `json:"length_red"`
		}
		var holes []HoleInfo
		for rows.Next() {
			var h HoleInfo
			if err := rows.Scan(&h.HoleNumber, &h.Par, &h.LengthYellow, &h.LengthRed); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			holes = append(holes, h)
		}
		json.NewEncoder(w).Encode(holes)
	} else if r.Method == http.MethodPost {
		type HoleInfo struct {
			HoleNumber   int `json:"hole_number"`
			Par          int `json:"par"`
			LengthYellow int `json:"length_yellow"`
			LengthRed    int `json:"length_red"`
		}
		var holes []HoleInfo
		if err := json.NewDecoder(r.Body).Decode(&holes); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		tx, err := db.DB.Begin()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, h := range holes {
			_, err := tx.Exec("UPDATE holes SET par = ?, length_yellow = ?, length_red = ? WHERE hole_number = ?", h.Par, h.LengthYellow, h.LengthRed, h.HoleNumber)
			if err != nil {
				tx.Rollback()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func ImportCourseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		http.Error(w, "Failed to parse CSV", http.StatusBadRequest)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for i, record := range records {
		// Skip header: Hole, Par, LengthYellow, LengthRed
		if i == 0 || len(record) < 3 {
			continue
		}

		hole, _ := strconv.Atoi(record[0])
		par, _ := strconv.Atoi(record[1])
		lengthYellow, _ := strconv.Atoi(record[2])
		lengthRed := lengthYellow // Default if not provided
		if len(record) > 3 {
			lengthRed, _ = strconv.Atoi(record[3])
		}

		if hole < 1 || hole > 18 {
			continue
		}

		_, err = tx.Exec("UPDATE holes SET par = ?, length_yellow = ?, length_red = ? WHERE hole_number = ?", par, lengthYellow, lengthRed, hole)
		if err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func ExportCourseHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query("SELECT hole_number, par, length_yellow, length_red FROM holes ORDER BY hole_number")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=course_config.csv")

	writer := csv.NewWriter(w)
	writer.Write([]string{"Hole", "Par", "LengthYellow", "LengthRed"})

	for rows.Next() {
		var hole, par, ly, lr int
		if err := rows.Scan(&hole, &par, &ly, &lr); err != nil {
			continue
		}
		writer.Write([]string{
			strconv.Itoa(hole),
			strconv.Itoa(par),
			strconv.Itoa(ly),
			strconv.Itoa(lr),
		})
	}
	writer.Flush()
}
func FetchHCPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Find players with handicap = 0 (or not set)
	rows, err := db.DB.Query("SELECT id, name, surname, reg_num FROM players WHERE (handicap = 0 OR handicap IS NULL) AND reg_num != ''")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var playersToFetch []models.Player
	for rows.Next() {
		var p models.Player
		if err := rows.Scan(&p.ID, &p.Name, &p.Surname, &p.RegNum); err != nil {
			continue
		}
		playersToFetch = append(playersToFetch, p)
	}

	if len(playersToFetch) == 0 {
		log.Println("No players to fetch HCP for")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	log.Printf("Starting HCP fetch for %d players", len(playersToFetch))

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for i, p := range playersToFetch {
		log.Printf("[%d/%d] Fetching HCP for %s %s (%s)...", i+1, len(playersToFetch), p.Name, p.Surname, p.RegNum)
		hcp, err := fetchSingleHCP(client, p.RegNum, p.Surname)
		if err == nil && hcp > 0 {
			log.Printf("Successfully fetched HCP %.1f for %s %s", hcp, p.Name, p.Surname)
			_, _ = db.DB.Exec("UPDATE players SET handicap = ? WHERE id = ?", hcp, p.ID)
		} else {
			log.Printf("Failed to fetch HCP for %s %s: %v", p.Name, p.Surname, err)
		}
		// Delay between players
		time.Sleep(1 * time.Second)
	}

	log.Println("HCP fetch process completed")
	w.WriteHeader(http.StatusOK)
}

func fetchSingleHCP(client *http.Client, regNum, surname string) (float64, error) {
	targetURL := "https://server.cgf.cz/HcpCheck.aspx"
	captchaCode := "z999d"

	for i := 0; i < 5; i++ { // Try up to 5 times
		if i > 0 {
			log.Printf("  - Retry %d/4 for captcha...", i)
		}
		// Step 1: GET to get session and viewstate
		resp, err := client.Get(targetURL)
		if err != nil {
			return 0, err
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			return 0, err
		}

		viewState, _ := doc.Find("#__VIEWSTATE").Attr("value")
		eventValidation, _ := doc.Find("#__EVENTVALIDATION").Attr("value")
		viewStateGen, _ := doc.Find("#__VIEWSTATEGENERATOR").Attr("value")

		// Step 2: POST with data
		data := url.Values{}
		data.Set("__VIEWSTATE", viewState)
		data.Set("__EVENTVALIDATION", eventValidation)
		data.Set("__VIEWSTATEGENERATOR", viewStateGen)
		data.Set("tbMemberNumber", regNum)
		data.Set("tbSurname", surname)
		data.Set("tbCaptcha", captchaCode)
		data.Set("btnCheck", "Ověřit")

		req, _ := http.NewRequest("POST", targetURL, strings.NewReader(data.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err = client.Do(req)
		if err != nil {
			return 0, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyStr := string(body)

		// Check if it's the result page or still the form
		if strings.Contains(bodyStr, "Aktuální hendikepový index") {
			// Parsing step
			doc, _ := goquery.NewDocumentFromReader(strings.NewReader(bodyStr))
			hcpStr := ""
			doc.Find("table tr").Each(func(i int, s *goquery.Selection) {
				if strings.Contains(s.Text(), "Aktuální hendikepový index") {
					hcpStr = s.Find("strong").First().Text()
				}
			})

			hcpStr = strings.TrimSpace(hcpStr)
			hcpStr = strings.Replace(hcpStr, ",", ".", -1) // Convert 9,4 to 9.4
			if hcp, err := strconv.ParseFloat(hcpStr, 64); err == nil {
				return hcp, nil
			}
		}

		// Wait 300ms before retry
		time.Sleep(300 * time.Millisecond)
	}

	return 0, fmt.Errorf("failed after retries")
}
