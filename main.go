package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const dateLayout = "20060102"

type appServer struct {
	db       *sql.DB
	password string
}

type task struct {
	ID      string `json:"id,omitempty"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

func main() {
	db, err := openDatabase(dbFile())
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	srv := &appServer{
		db:       db,
		password: os.Getenv("TODO_PASSWORD"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/signin", srv.signIn)
	mux.Handle("/api/", srv.auth(http.HandlerFunc(srv.api)))
	mux.Handle("/", srv.static())

	addr := ":" + port()
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func port() string {
	if value := os.Getenv("TODO_PORT"); value != "" {
		if _, err := strconv.Atoi(value); err == nil {
			return value
		}
	}
	return "7540"
}

func dbFile() string {
	if value := os.Getenv("TODO_DBFILE"); value != "" {
		return value
	}
	return "scheduler.db"
}

func openDatabase(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec(`
CREATE TABLE IF NOT EXISTS scheduler (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	date TEXT NOT NULL,
	title TEXT NOT NULL,
	comment TEXT NOT NULL DEFAULT '',
	repeat TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS scheduler_date_id ON scheduler(date, id);
`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (s *appServer) static() http.Handler {
	files := http.FileServer(http.Dir("web"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.password != "" && (r.URL.Path == "/" || r.URL.Path == "/index.html") && !s.isAuthenticated(r) {
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}
		files.ServeHTTP(w, r)
	})
}

func (s *appServer) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.password != "" && !s.isAuthenticated(r) {
			writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *appServer) isAuthenticated(r *http.Request) bool {
	if s.password == "" {
		return true
	}
	cookie, err := r.Cookie("token")
	if err != nil || cookie.Value == "" {
		return false
	}
	return verifyToken(cookie.Value, s.password)
}

func (s *appServer) api(w http.ResponseWriter, r *http.Request) {
	switch strings.TrimPrefix(r.URL.Path, "/api/") {
	case "nextdate":
		s.nextDate(w, r)
	case "tasks":
		s.tasks(w, r)
	case "task":
		s.task(w, r)
	case "task/done":
		s.done(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *appServer) signIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]string{"error": "invalid request"})
		return
	}
	if s.password == "" || req.Password != s.password {
		writeJSON(w, map[string]string{"error": "invalid password"})
		return
	}
	token, err := makeToken(s.password)
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": "cannot create token"})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		MaxAge:   int((8 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, map[string]string{"token": token})
}

func makeToken(secret string) (string, error) {
	header, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(secret))
	claims, err := json.Marshal(map[string]any{
		"hash": hex.EncodeToString(hash[:]),
		"exp":  time.Now().Add(8 * time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyToken(token, secret string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	unsigned := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return false
	}
	if exp, ok := claims["exp"]; ok {
		return expirationValid(exp)
	}
	return true
}

func expirationValid(exp any) bool {
	var unix int64
	switch value := exp.(type) {
	case float64:
		unix = int64(value)
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return false
		}
		unix = parsed
	default:
		return false
	}
	return time.Now().Unix() < unix
}

func (s *appServer) nextDate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	now, err := time.Parse(dateLayout, r.URL.Query().Get("now"))
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	next, err := nextDate(now, r.URL.Query().Get("date"), r.URL.Query().Get("repeat"))
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(next))
}

func (s *appServer) tasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rows, err := s.queryTasks(r.URL.Query().Get("search"))
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"tasks": rows})
}

func (s *appServer) queryTasks(search string) ([]task, error) {
	const limit = 50
	var rows *sql.Rows
	var err error
	search = strings.TrimSpace(search)
	if search == "" {
		rows, err = s.db.Query(`SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date, id LIMIT ?`, limit)
	} else if date, ok := parseSearchDate(search); ok {
		rows, err = s.db.Query(`SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date, id LIMIT ?`, date, limit)
	} else {
		like := "%" + search + "%"
		rows, err = s.db.Query(`SELECT id, date, title, comment, repeat FROM scheduler WHERE title LIKE ? OR comment LIKE ? ORDER BY date, id LIMIT ?`, like, like, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]task, 0)
	for rows.Next() {
		var item task
		if err := rows.Scan(&item.ID, &item.Date, &item.Title, &item.Comment, &item.Repeat); err != nil {
			return nil, err
		}
		tasks = append(tasks, item)
	}
	return tasks, rows.Err()
}

func parseSearchDate(value string) (string, bool) {
	date, err := time.Parse("02.01.2006", value)
	if err != nil {
		return "", false
	}
	return date.Format(dateLayout), true
}

func (s *appServer) task(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getTask(w, r)
	case http.MethodPost:
		s.addTask(w, r)
	case http.MethodPut:
		s.updateTask(w, r)
	case http.MethodDelete:
		s.deleteTask(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *appServer) getTask(w http.ResponseWriter, r *http.Request) {
	id, err := queryID(r)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	item, err := s.loadTask(id)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, item)
}

func (s *appServer) addTask(w http.ResponseWriter, r *http.Request) {
	item, err := decodeTask(r)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	if err := normalizeTask(&item); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	res, err := s.db.Exec(`INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)`, item.Date, item.Title, item.Comment, item.Repeat)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]string{"id": strconv.FormatInt(id, 10)})
}

func (s *appServer) updateTask(w http.ResponseWriter, r *http.Request) {
	item, err := decodeTask(r)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	id, err := strconv.ParseInt(item.ID, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, map[string]string{"error": "invalid id"})
		return
	}
	if err := normalizeTask(&item); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	res, err := s.db.Exec(`UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat = ? WHERE id = ?`, item.Date, item.Title, item.Comment, item.Repeat, id)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		writeJSON(w, map[string]string{"error": "task not found"})
		return
	}
	writeJSON(w, map[string]any{})
}

func (s *appServer) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := queryID(r)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	res, err := s.db.Exec(`DELETE FROM scheduler WHERE id = ?`, id)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		writeJSON(w, map[string]string{"error": "task not found"})
		return
	}
	writeJSON(w, map[string]any{})
}

func (s *appServer) done(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id, err := queryID(r)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	item, err := s.loadTask(id)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	if item.Repeat == "" {
		if _, err := s.db.Exec(`DELETE FROM scheduler WHERE id = ?`, id); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{})
		return
	}
	next, err := nextDate(time.Now(), item.Date, item.Repeat)
	if err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	if _, err := s.db.Exec(`UPDATE scheduler SET date = ? WHERE id = ?`, next, id); err != nil {
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{})
}

func decodeTask(r *http.Request) (task, error) {
	var item task
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		return item, errors.New("invalid request")
	}
	item.ID = strings.TrimSpace(item.ID)
	item.Date = strings.TrimSpace(item.Date)
	item.Title = strings.TrimSpace(item.Title)
	item.Comment = strings.TrimSpace(item.Comment)
	item.Repeat = strings.TrimSpace(item.Repeat)
	return item, nil
}

func normalizeTask(item *task) error {
	if item.Title == "" {
		return errors.New("title is required")
	}
	today := truncateDate(time.Now())
	if item.Date == "" {
		item.Date = today.Format(dateLayout)
	}
	date, err := time.Parse(dateLayout, item.Date)
	if err != nil || date.Format(dateLayout) != item.Date {
		return errors.New("invalid date")
	}
	if item.Repeat != "" {
		if _, err := nextDate(today, item.Date, item.Repeat); err != nil {
			return err
		}
	}
	if !date.Before(today) {
		return nil
	}
	if item.Repeat == "" {
		item.Date = today.Format(dateLayout)
		return nil
	}
	item.Date, err = nextDate(today, item.Date, item.Repeat)
	return err
}

func truncateDate(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func (s *appServer) loadTask(id int64) (task, error) {
	var item task
	err := s.db.QueryRow(`SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?`, id).
		Scan(&item.ID, &item.Date, &item.Title, &item.Comment, &item.Repeat)
	if errors.Is(err, sql.ErrNoRows) {
		return item, errors.New("task not found")
	}
	return item, err
}

func queryID(r *http.Request) (int64, error) {
	value := strings.TrimSpace(r.URL.Query().Get("id"))
	if value == "" {
		return 0, errors.New("id is required")
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func nextDate(now time.Time, dateValue, repeat string) (string, error) {
	date, err := time.Parse(dateLayout, dateValue)
	if err != nil || date.Format(dateLayout) != dateValue {
		return "", errors.New("invalid date")
	}
	fields := strings.Fields(repeat)
	if len(fields) == 0 {
		return "", errors.New("empty repeat")
	}
	now = truncateDate(now)
	switch fields[0] {
	case "d":
		return nextDaily(now, date, fields)
	case "y":
		if len(fields) != 1 {
			return "", errors.New("invalid yearly repeat")
		}
		next := date.AddDate(1, 0, 0)
		for !next.After(now) {
			next = next.AddDate(1, 0, 0)
		}
		return next.Format(dateLayout), nil
	case "w":
		return nextWeekly(now, date, fields)
	case "m":
		return nextMonthly(now, date, fields)
	default:
		return "", errors.New("invalid repeat")
	}
}

func nextDaily(now, date time.Time, fields []string) (string, error) {
	if len(fields) != 2 {
		return "", errors.New("invalid daily repeat")
	}
	days, err := strconv.Atoi(fields[1])
	if err != nil || days < 1 || days > 400 {
		return "", errors.New("invalid daily repeat")
	}
	next := date.AddDate(0, 0, days)
	for !next.After(now) {
		next = next.AddDate(0, 0, days)
	}
	return next.Format(dateLayout), nil
}

func nextWeekly(now, date time.Time, fields []string) (string, error) {
	if len(fields) != 2 {
		return "", errors.New("invalid weekly repeat")
	}
	days, err := parseIntList(fields[1])
	if err != nil || len(days) == 0 {
		return "", errors.New("invalid weekly repeat")
	}
	allowed := make(map[int]bool, len(days))
	for _, day := range days {
		if day < 1 || day > 7 {
			return "", errors.New("invalid weekly repeat")
		}
		allowed[day] = true
	}
	next := date.AddDate(0, 0, 1)
	deadline := maxTime(now, date).AddDate(5, 0, 0)
	for !next.After(deadline) {
		if next.After(now) && allowed[weekdayNumber(next)] {
			return next.Format(dateLayout), nil
		}
		next = next.AddDate(0, 0, 1)
	}
	return "", errors.New("weekly repeat not found")
}

func weekdayNumber(date time.Time) int {
	day := int(date.Weekday())
	if day == 0 {
		return 7
	}
	return day
}

func nextMonthly(now, date time.Time, fields []string) (string, error) {
	if len(fields) < 2 || len(fields) > 3 {
		return "", errors.New("invalid monthly repeat")
	}
	days, err := parseIntList(fields[1])
	if err != nil || len(days) == 0 {
		return "", errors.New("invalid monthly repeat")
	}
	for _, day := range days {
		if day == 0 || day < -2 || day > 31 {
			return "", errors.New("invalid monthly repeat")
		}
	}
	months := map[int]bool{}
	if len(fields) == 3 {
		parsed, err := parseIntList(fields[2])
		if err != nil || len(parsed) == 0 {
			return "", errors.New("invalid monthly repeat")
		}
		for _, month := range parsed {
			if month < 1 || month > 12 {
				return "", errors.New("invalid monthly repeat")
			}
			months[month] = true
		}
	}
	sort.Ints(days)
	next := date.AddDate(0, 0, 1)
	deadline := maxTime(now, date).AddDate(5, 0, 0)
	for !next.After(deadline) {
		if next.After(now) && monthMatches(next, months) && dayMatches(next, days) {
			return next.Format(dateLayout), nil
		}
		next = next.AddDate(0, 0, 1)
	}
	return "", errors.New("monthly repeat not found")
}

func parseIntList(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errors.New("empty value")
		}
		number, err := strconv.Atoi(part)
		if err != nil {
			return nil, err
		}
		result = append(result, number)
	}
	return result, nil
}

func monthMatches(date time.Time, months map[int]bool) bool {
	return len(months) == 0 || months[int(date.Month())]
}

func dayMatches(date time.Time, days []int) bool {
	last := daysInMonth(date.Year(), date.Month())
	for _, day := range days {
		switch day {
		case -1:
			if date.Day() == last {
				return true
			}
		case -2:
			if date.Day() == last-1 {
				return true
			}
		default:
			if date.Day() == day {
				return true
			}
		}
	}
	return false
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func writeJSON(w http.ResponseWriter, value any) {
	writeJSONStatus(w, http.StatusOK, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		_, _ = fmt.Fprintf(w, `{"error":%q}`, err.Error())
	}
}
