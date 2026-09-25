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
	"github.com/josejuanmontiel/primaryfoods/internal/config"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

func main() {
	configFile := flag.String("config", "config.json", "Ruta del archivo de configuración JSON")
	portFlag := flag.String("port", "", "Puerto HTTP del servidor (anula config.json si se indica)")
	dbPathFlag := flag.String("db", "", "Ruta de la base de datos SQLite (anula config.json si se indica)")
	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Printf("⚠️ Error cargando config desde %s: %v. Usando valores por defecto.", *configFile, err)
		cfg = config.DefaultConfig()
	}

	if *portFlag != "" {
		cfg.Port = *portFlag
	}
	if *dbPathFlag != "" {
		cfg.DBPath = *dbPathFlag
	}

	log.Printf("🥦 Inicializando PrimaryFoods...")
	log.Printf("⚙️  Configuración: %s (Mealie: %s)", *configFile, cfg.MealieURL)
	log.Printf("📦 Base de datos: %s", cfg.DBPath)

	store, err := storage.NewSQLiteStorage(cfg.DBPath)
	if err != nil {
		log.Fatalf("❌ Error iniciando base de datos: %v", err)
	}
	defer store.Close()

	srv := api.NewServer(store, cfg)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
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
		fmt.Printf("🚀 PrimaryFoods API corriendo en: http://localhost:%s\n", cfg.Port)
		fmt.Println("──────────────────────────────────────────────────────────")
		fmt.Printf("  • Healthcheck:        GET http://localhost:%s/api/health\n", cfg.Port)
		fmt.Printf("  • Listar alimentos:   GET http://localhost:%s/api/foods\n", cfg.Port)
		fmt.Printf("  • Buscar (SmartMatch):GET http://localhost:%s/api/foods/search?q=brocoli\n", cfg.Port)
		fmt.Printf("  • Exportar JSON:      GET http://localhost:%s/api/export/json\n", cfg.Port)
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
