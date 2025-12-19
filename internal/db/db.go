package db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(filepath string) {
	var err error
	DB, err = sql.Open("sqlite3", filepath)
	if err != nil {
		log.Fatal(err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatal(err)
	}

	createTables()
}

func createTables() {
	createPlayersTable := `CREATE TABLE IF NOT EXISTS players (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		surname TEXT,
		reg_num TEXT,
		handicap REAL,
		gender TEXT DEFAULT 'M'
	);`

	createFlightsTable := `CREATE TABLE IF NOT EXISTS flights (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		token TEXT UNIQUE,
		name TEXT,
		starting_hole INTEGER DEFAULT 1
	);`

	createFlightPlayersTable := `CREATE TABLE IF NOT EXISTS flight_players (
		flight_id INTEGER,
		player_id INTEGER,
		FOREIGN KEY(flight_id) REFERENCES flights(id),
		FOREIGN KEY(player_id) REFERENCES players(id)
	);`

	createScoresTable := `CREATE TABLE IF NOT EXISTS scores (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		player_id INTEGER,
		hole_number INTEGER,
		strokes INTEGER,
		FOREIGN KEY(player_id) REFERENCES players(id)
	);`

	createHolesTable := `CREATE TABLE IF NOT EXISTS holes (
		hole_number INTEGER PRIMARY KEY,
		par INTEGER,
		length_yellow INTEGER DEFAULT 0,
		length_red INTEGER DEFAULT 0
	);`

	_, err := DB.Exec(createPlayersTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = DB.Exec(createFlightsTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = DB.Exec(createFlightPlayersTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = DB.Exec(createScoresTable)
	if err != nil {
		log.Fatal(err)
	}

	_, err = DB.Exec(createHolesTable)
	if err != nil {
		log.Fatal(err)
	}

	// Migrations: Add length if it doesn't exist
	_, _ = DB.Exec("ALTER TABLE holes ADD COLUMN length_red INTEGER DEFAULT 0")
	_, _ = DB.Exec("ALTER TABLE holes RENAME COLUMN length TO length_yellow")
	_, _ = DB.Exec("ALTER TABLE flights ADD COLUMN starting_hole INTEGER DEFAULT 1")
	_, _ = DB.Exec("ALTER TABLE players ADD COLUMN gender TEXT DEFAULT 'M'")

	// Populate holes if empty
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM holes").Scan(&count)
	if count == 0 {
		for i := 1; i <= 18; i++ {
			DB.Exec("INSERT INTO holes (hole_number, par, length_yellow, length_red) VALUES (?, ?, ?, ?)", i, 4, 300, 250)
		}
	}
}
