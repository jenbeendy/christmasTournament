package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/antigravity/christmasTournament/internal/db"
	"github.com/antigravity/christmasTournament/internal/models"
)

func PlayersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := db.DB.Query("SELECT id, name, surname, reg_num, handicap FROM players")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var players []models.Player
		for rows.Next() {
			var p models.Player
			if err := rows.Scan(&p.ID, &p.Name, &p.Surname, &p.RegNum, &p.Handicap); err != nil {
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
			_, err := db.DB.Exec("UPDATE players SET name=?, surname=?, reg_num=?, handicap=? WHERE id=?", p.Name, p.Surname, p.RegNum, p.Handicap, p.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		} else {
			// Create
			_, err := db.DB.Exec("INSERT INTO players (name, surname, reg_num, handicap) VALUES (?, ?, ?, ?)", p.Name, p.Surname, p.RegNum, p.Handicap)
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
		if len(record) < 4 {
			continue // Skip invalid rows
		}

		name := record[0]
		surname := record[1]
		regNum := record[2]
		handicap, err := strconv.ParseFloat(record[3], 64)
		if err != nil {
			continue // Skip invalid handicap
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
			SELECT f.id, f.token, f.name, p.id, p.name, p.surname, p.reg_num, p.handicap
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
			ID      int             `json:"id"`
			Token   string          `json:"token"`
			Name    string          `json:"name"`
			Players []models.Player `json:"players"`
		})

		for rows.Next() {
			var fID int
			var fToken, fName string
			var pID sql.NullInt64
			var pName, pSurname, pRegNum sql.NullString
			var pHandicap sql.NullFloat64

			if err := rows.Scan(&fID, &fToken, &fName, &pID, &pName, &pSurname, &pRegNum, &pHandicap); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if _, ok := flightMap[fID]; !ok {
				flightMap[fID] = &struct {
					ID      int             `json:"id"`
					Token   string          `json:"token"`
					Name    string          `json:"name"`
					Players []models.Player `json:"players"`
				}{
					ID:      fID,
					Token:   fToken,
					Name:    fName,
					Players: []models.Player{},
				}
			}

			if pID.Valid {
				flightMap[fID].Players = append(flightMap[fID].Players, models.Player{
					ID:       int(pID.Int64),
					Name:     pName.String,
					Surname:  pSurname.String,
					RegNum:   pRegNum.String,
					Handicap: pHandicap.Float64,
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
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Generate a hash token
		hash := sha256.Sum256([]byte(req.Name + time.Now().String()))
		token := hex.EncodeToString(hash[:])[:16] // Take first 16 chars

		res, err := db.DB.Exec("INSERT INTO flights (token, name) VALUES (?, ?)", token, req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "token": token})
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
		SELECT p.id, p.name, p.surname, p.handicap, SUM(s.strokes) as total_strokes
		FROM players p
		JOIN scores s ON p.id = s.player_id
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
		var totalStrokes int
		if err := rows.Scan(&pID, &pName, &pSurname, &pHandicap, &totalStrokes); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		netScore := float64(totalStrokes) - pHandicap
		results = append(results, map[string]interface{}{
			"player_id": pID,
			"name":      pName,
			"surname":   pSurname,
			"handicap":  pHandicap,
			"gross":     totalStrokes,
			"net":       netScore,
		})
	}
	json.NewEncoder(w).Encode(results)
}

func CourseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := db.DB.Query("SELECT hole_number, par FROM holes ORDER BY hole_number")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var holes []struct {
			HoleNumber int `json:"hole_number"`
			Par        int `json:"par"`
		}
		for rows.Next() {
			var h struct {
				HoleNumber int `json:"hole_number"`
				Par        int `json:"par"`
			}
			if err := rows.Scan(&h.HoleNumber, &h.Par); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			holes = append(holes, h)
		}
		json.NewEncoder(w).Encode(holes)
	} else if r.Method == http.MethodPost {
		var holes []struct {
			HoleNumber int `json:"hole_number"`
			Par        int `json:"par"`
		}
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
			_, err := tx.Exec("UPDATE holes SET par = ? WHERE hole_number = ?", h.Par, h.HoleNumber)
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
