package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"token_transfer/graph"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Build DB connection string
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
	)
	fmt.Println(connStr)

	// Open DB connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error connecting to DB:", err)
	}

	// Close connection when main() finishes
	defer db.Close()

	// Check if DB is reachable
	if err := db.Ping(); err != nil {
		log.Fatal("Ping failed:", err)
	}

	fmt.Println("Connected to DB.")

	// Start Graph server
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "wallets",
	}

	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.Use(extension.Introspection{})

	http.Handle("/", playground.Handler("GraphQL", "/query"))
	http.Handle("/query", srv)

	log.Println("GraphQL server running at http://localhost:8080/")
	log.Fatal(http.ListenAndServe(":8080", nil))

}
