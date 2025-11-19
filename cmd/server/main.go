package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"flash-ai/internal/api"
	"flash-ai/internal/config"
	"flash-ai/internal/db"
	"flash-ai/internal/services"
)

func main() {
	cfg := config.Load()

	conn, err := db.Open(cfg.Database)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer conn.Close()

	flashcardService := services.NewFlashcardService(conn)
	conceptService := services.NewConceptService(conn)
	documentService := services.NewDocumentService(conn, cfg.UploadDir)
	pdfService := services.NewPDFService()
	aiService := services.NewAIService(
		cfg.OpenAIKey,
		cfg.OpenAIModel,
		cfg.OpenAIEndpoint,
		cfg.ZAIKey,
		cfg.ZAIBaseURL,
		cfg.ZAIModel,
		pdfService,
	)
	ingestionService := services.NewIngestionService(documentService, pdfService, aiService, flashcardService, conceptService)

	server := api.NewServer(flashcardService, conceptService, documentService, ingestionService)
	mux := http.NewServeMux()

	assetsFS := http.FileServer(http.Dir("./internal/web/assets"))
	mux.Handle("/assets/", http.StripPrefix("/assets/", assetsFS))

	staticFS := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", staticFS))

	mux.HandleFunc("/", serveFile("./internal/web/index.html"))
	mux.HandleFunc("/review", serveFile("./internal/web/review.html"))
	mux.HandleFunc("/upload", serveFile("./internal/web/upload.html"))
	mux.HandleFunc("/topics", serveFile("./internal/web/topics.html"))
	mux.HandleFunc("/flashcards", serveFile("./internal/web/flashcards.html"))

	mux.Handle("/api", server.Handler())
	mux.Handle("/api/", server.Handler())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("listening on :%s", port)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func serveFile(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.ServeFile(w, r, path)
	}
}
