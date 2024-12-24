
package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "sync/atomic"
    "time"

    "github.com/gorilla/mux"
    _ "github.com/lib/pq"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

var (
    db *sql.DB
    isReady atomic.Bool
)

func init() {
    // Configure zerolog
    zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
    log.Logger = log.Output(zerolog.ConsoleWriter{
        Out:        os.Stdout,
        TimeFormat: time.RFC3339,
    })
}

func main() {
    log.Info().Msg("Starting Drizzle API service")
    isReady.Store(false)

    // Get database credentials from environment variables
    dbUser := os.Getenv("DB_USER")
    dbPassword := os.Getenv("DB_PASSWORD")
    dbName := os.Getenv("DB_NAME")
    dbPort := os.Getenv("DB_PORT")
    
    if dbPort == "" {
        dbPort = "5432"
        log.Info().Msg("No DB_PORT specified, using default: 5432")
    }

    // Construct connection string
    connStr := fmt.Sprintf(
        "host=127.0.0.1 port=%s user=%s password=%s dbname=%s sslmode=disable",
         dbPort, dbUser, dbPassword, dbName,
    )

    log.Info().
        Str("host", "127.0.0.1").
        Str("port", dbPort).
        Str("db_name", dbName).
        Msg("Attempting database connection")

    var err error
    db, err = sql.Open("postgres", connStr)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to open database connection")
    }
    defer db.Close()

    // Test database connection
    err = db.Ping()
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to ping database")
    }
    log.Info().Msg("Successfully connected to database")

    isReady.Store(true)

    // Router setup
    r := mux.NewRouter()
    s := r.PathPrefix("/drizzlego").Subrouter()
    
    // Register routes
    s.HandleFunc("/isActive", healthCheckHandler).Methods("GET")
    s.HandleFunc("/data", createUserHandler).Methods("POST")
    s.HandleFunc("/health/live", livenessHandler).Methods("GET")
    s.HandleFunc("/health/ready", readinessHandler).Methods("GET")

    // Add logging middleware
    r.Use(loggingMiddleware)

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
        log.Info().Msg("No PORT specified, using default: 8080")
    }

    log.Info().Str("port", port).Msg("Server starting")
    if err := http.ListenAndServe(":"+port, r); err != nil {
        log.Fatal().Err(err).Msg("Server failed to start")
    }
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        startTime := time.Now()

        log.Info().
            Str("method", r.Method).
            Str("path", r.URL.Path).
            Str("remote_addr", r.RemoteAddr).
            Msg("Request received")

        next.ServeHTTP(w, r)

        log.Info().
            Str("method", r.Method).
            Str("path", r.URL.Path).
            Dur("duration", time.Since(startTime)).
            Msg("Request completed")
    })
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
    log.Debug().Msg("Liveness check called")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
    if !isReady.Load() {
        log.Warn().Msg("Readiness check failed: application not ready")
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
        return
    }

    if err := db.Ping(); err != nil {
        log.Error().Err(err).Msg("Readiness check failed: database not responding")
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{"status": "database not ready"})
        return
    }

    log.Debug().Msg("Readiness check passed")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
    log.Debug().Msg("Health check called")
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"message": "Welcome to Drizzle"})
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
    var user User
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        log.Error().Err(err).Msg("Failed to decode request body")
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    log.Info().
        Str("user_id", user.ID).
        Str("user_name", user.Name).
        Msg("Attempting to create new user")

    _, err := db.Exec("INSERT INTO tablea (id, name) VALUES ($1, $2)", user.ID, user.Name)
    if err != nil {
        log.Error().
            Err(err).
            Str("user_id", user.ID).
            Str("user_name", user.Name).
            Msg("Failed to insert user into database")
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    log.Info().
        Str("user_id", user.ID).
        Str("user_name", user.Name).
        Msg("Successfully created new user")

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}