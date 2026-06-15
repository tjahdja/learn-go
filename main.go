package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"my-server/internal/config"
	"my-server/internal/database"
	"my-server/internal/pkg/middleware"
	"my-server/internal/todo"
	"my-server/internal/user"
	"net/http"
	"os"
	"os/signal"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// This is our 'Handler' function.
// 'w' is where we write the data we want to send back to the user.
// 'r' contains everything the user sent us (URL, headers, etc.)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to the home page!")
}

func main() {

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Starting server initialization...")
	cfg := config.LoadConfig()
	db, err := sql.Open("pgx", cfg.DBConn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("Migration runner halted startup: %v", err)
	}

	todoStore := todo.NewStore(db)
	userStore := user.NewStore(db)

	// 1. Create a "ServeMux" (Multiplexer).
	// This is the "Router." Its only job is to look at the URL
	// and decide which handler function should deal with it.
	mux := http.NewServeMux()
	protectedRouter := middleware.Recovery(mux)
	limitedRouter := middleware.RateLimiter(protectedRouter)
	loggedMux := middleware.StructuredLogger(limitedRouter)

	// 2. Register our route.
	// When someone visits "/", run the 'homeHandler' function.
	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("/user/register", userStore.RegisterHandler)
	mux.HandleFunc("/user/login", userStore.LoginHandler)
	mux.Handle("/todo", middleware.AuthMiddleware(http.HandlerFunc(todoStore.TodoHandler)))

	server := &http.Server{
		Addr:    ":8080",
		Handler: loggedMux,
	}

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt)

	go func() {
		slog.Info("Server listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", slog.Any("error", err))
		}
	}()
	sig := <-shutdownSignals
	fmt.Printf("\nReceived signal: %v. Starting graceful shutdown...\n", sig)

	// 5. Create a context with a 5-second timeout safety net
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 6. Tell the HTTP server to shed its load gracefully
	if err := server.Shutdown(ctx); err != nil {
		fmt.Printf("Server forced to close: %v\n", err)
	}

	// 7. Clean up other dependencies safely
	fmt.Println("Closing database connections...")
	db.Close()

	fmt.Println("Server successfully stopped completely.")
}
