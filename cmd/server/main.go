package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/josejuanmontiel/primaryfoods/internal/api"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

func main() {
	defaultPort := "8086"
	if p := os.Getenv("PORT"); p != "" {
		defaultPort = p
	}

	defaultDB := "data/primaryfoods.db"
	if d := os.Getenv("DB_PATH"); d != "" {
		defaultDB = d
	}

	port := flag.String("port", defaultPort, "Puerto HTTP del servidor")
	dbPath := flag.String("db", defaultDB, "Ruta del archivo de base de datos SQLite (:memory: para memoria)")
	flag.Parse()

	log.Printf("🥦 Inicializando PrimaryFoods...")
	log.Printf("📦 Base de datos: %s", *dbPath)

	store, err := storage.NewSQLiteStorage(*dbPath)
	if err != nil {
		log.Fatalf("❌ Error iniciando base de datos: %v", err)
	}
	defer store.Close()

	srv := api.NewServer(store)

	httpServer := &http.Server{
		Addr:         ":" + *port,
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Canal para captura de señales de apagado gracioso
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Println("──────────────────────────────────────────────────────────")
		fmt.Printf("🚀 PrimaryFoods API corriendo en: http://localhost:%s\n", *port)
		fmt.Println("──────────────────────────────────────────────────────────")
		fmt.Printf("  • Healthcheck:        GET http://localhost:%s/api/health\n", *port)
		fmt.Printf("  • Listar alimentos:   GET http://localhost:%s/api/foods\n", *port)
		fmt.Printf("  • Buscar (SmartMatch):GET http://localhost:%s/api/foods/search?q=brocoli\n", *port)
		fmt.Printf("  • Exportar JSON:      GET http://localhost:%s/api/export/json\n", *port)
		fmt.Println("──────────────────────────────────────────────────────────")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Error en servidor HTTP: %v", err)
		}
	}()

	<-stopChan
	log.Println("\n🛑 Apagando servidor PrimaryFoods...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("⚠️  Error en apagado gracioso: %v", err)
	}

	log.Println("✅ Servidor detenido correctamente.")
}
