// src/main.go
package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "sync/atomic"

    "github.com/gorilla/mux"
    _ "github.com/lib/pq"
)

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

var (
    db *sql.DB
    isReady atomic.Bool
)

func main() {
    isReady.Store(false)

    // Get database credentials from environment variables
    dbUser := os.Getenv("DB_USER")
    dbPassword := os.Getenv("DB_PASSWORD")
    dbName := os.Getenv("DB_NAME")
    dbPort := os.Getenv("DB_PORT")
    
    if dbPort == "" {
        dbPort = "5432"
    }

    // Construct connection string
    connStr := fmt.Sprintf(
        "host=127.0.0.1 port=%s user=%s password=%s dbname=%s sslmode=disable",
         dbPort, dbUser, dbPassword, dbName,
    )

    var err error
    db, err = sql.Open("postgres", connStr)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Test database connection
    err = db.Ping()
    if err != nil {
        log.Fatal(err)
    }

    isReady.Store(true)

    // Router setup
    r := mux.NewRouter()
    
    // Create a subrouter with the prefix
    s := r.PathPrefix("/drizzlego").Subrouter()
    
    // Main application endpoints
    s.HandleFunc("/isActive", healthCheckHandler).Methods("GET")
    s.HandleFunc("/data", createUserHandler).Methods("POST")
    
    // Health check endpoints
    s.HandleFunc("/health/live", livenessHandler).Methods("GET")
    s.HandleFunc("/health/ready", readinessHandler).Methods("GET")

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    log.Printf("Server starting on port %s", port)
    if err := http.ListenAndServe(":"+port, r); err != nil {
        log.Fatal(err)
    }
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
    if !isReady.Load() {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
        return
    }

    if err := db.Ping(); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{"status": "database not ready"})
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"message": "Welcome to Drizzle"})
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
    var user User
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    _, err := db.Exec("INSERT INTO tablea (id, name) VALUES ($1, $2)", user.ID, user.Name)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}