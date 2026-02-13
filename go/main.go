/*copyright Abdel-Karim Al Chaer*/
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql" // MYSQL Treiber
)

// Server hält die DB-Verbindung
type Server struct{ DB *sql.DB }

func main() {
	//  Verbindung & Port
	dsn := mustEnv("MYSQL_DSN") // Checkt ob umgebungsvariable gesetzt ist
	port := env("PORT", "8080") // Nimmt Parameter für port oder Standard 8080
	db := mustOpen(dsn)         // öffnet & pingt DB sonst abbruch
	s := &Server{DB: db}        // Server-Instanz mit DB

	// Routen registrieren
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok")) // Check ob Server läuft
	})
	http.HandleFunc("/api/checkin", s.checkin)   // POST
	http.HandleFunc("/api/checkout", s.checkout) // POST
	http.HandleFunc("/api/present", s.present)   // GET

	//  Server starten
	log.Println("API auf http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil)) // Server start
}

func (s *Server) checkin(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost { //Methode post?
		http.Error(w, "Methode nicht erlaubt", 405)
		return
	}
	var in struct{ FirstName, LastName string } // Auslesen von JSon
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	fn := strings.TrimSpace(in.FirstName) // Trim
	ln := strings.TrimSpace(in.LastName)
	if fn == "" || ln == "" { // Check ob ausgefüllt
		http.Error(w, "first_name/last_name required", 400)
		return
	}

	// Person-ID besorgen falls nicht vorhanden neu anlegen
	pid, err := s.getOrCreatePerson(fn, ln)
	if err != nil {
		http.Error(w, "db error("+err.Error()+")", 500)
		return
	}

	//  Prüfen, ob bereits checkin, damit doppelte checkins nicht gehen
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE person_id=? AND checkout_at IS NULL`, pid).Scan(&count); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if count > 0 {
		http.Error(w, "already checked in", 409)
		return
	}

	//  Einchecken
	if _, err := s.DB.Exec(`INSERT INTO sessions(person_id) VALUES (?)`, pid); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	writeJSON(w, map[string]any{"status": "checked_in", "person_id": pid}) // Any für beliebigen Typ
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) {
	// Erwartet JSON: {"first_name":"Max","last_name":"Mustermann"}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct{ FirstName, LastName string }
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	fn := strings.TrimSpace(in.FirstName)
	ln := strings.TrimSpace(in.LastName)
	if fn == "" || ln == "" {
		http.Error(w, "first_name/last_name required", 400)
		return
	}

	//  Person finden (beim Checkout NICHT automatisch anlegen)
	pid, err := s.findPerson(fn, ln)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "person not found", 404)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	//  Offene Session holen
	var sid uint64
	err = s.DB.QueryRow(`SELECT id FROM sessions WHERE person_id=? AND checkout_at IS NULL ORDER BY id DESC LIMIT 1`, pid).Scan(&sid)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "not checked in", 409)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	//  Auschecken
	if _, err := s.DB.Exec(`UPDATE sessions SET checkout_at=NOW() WHERE id=?`, sid); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	writeJSON(w, map[string]any{"status": "checked_out", "person_id": pid})
}

func (s *Server) present(w http.ResponseWriter, r *http.Request) {
	// Liste aller offenen Sessions (wer ist drin)
	type Row struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		CheckinAt string `json:"checkin_at"`
	}
	rows, err := s.DB.Query(`
		SELECT p.first_name, p.last_name, DATE_FORMAT(s.checkin_at, "%Y-%m-%d %H:%i:%s")
		FROM sessions s
		JOIN people p ON p.id = s.person_id 
		WHERE s.checkout_at IS NULL
		ORDER BY s.checkin_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var out []Row
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.FirstName, &r.LastName, &r.CheckinAt); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		out = append(out, r)
	}
	writeJSON(w, out)
}

// ===== DB-Helfer =====

func (s *Server) getOrCreatePerson(fn, ln string) (uint64, error) {
	//  Versuchen zu finden
	if id, err := s.findPerson(fn, ln); err == nil {
		return id, nil
	}
	// 2) Sonst anlegen
	res, err := s.DB.Exec(`INSERT INTO people(first_name,last_name) VALUES(?,?)`, fn, ln)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil

}

func (s *Server) findPerson(fn, ln string) (uint64, error) {
	var id uint64
	err := s.DB.QueryRow(`SELECT id FROM people WHERE first_name=? AND last_name=?`, fn, ln).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Werkzeuge

// funktion um json auszugeben
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("%s missing", k)
	}
	return v
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func mustOpen(dsn string) *sql.DB {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("DB nicht erreichbar:", err)
	}
	return db
}
