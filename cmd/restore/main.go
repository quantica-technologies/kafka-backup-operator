package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/quantica-technologies/kafka-backup/internal/app/restore"
	"github.com/quantica-technologies/kafka-backup/internal/config"
	"github.com/quantica-technologies/kafka-backup/internal/infrastructure/kafka"
	"github.com/quantica-technologies/kafka-backup/internal/infrastructure/storage"
	"github.com/quantica-technologies/kafka-backup/pkg/logger"
)

const version = "1.0.0"

func main() {
	var (
		configFile  = flag.String("config", "config.yaml", "Path to configuration file")
		backupID    = flag.String("backup-id", "", "Backup ID to restore")
		showVersion = flag.Bool("version", false, "Show version")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Kafka Restore v%s\n", version)
		return
	}

	if *backupID == "" {
		fmt.Fprintf(os.Stderr, "Backup ID is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.LoadFromFile(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New(logger.Config{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	})

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Received shutdown signal")
		cancel()
	}()

	// Initialize repositories
	kafkaRepo := kafka.NewRepository()
	storageRepo, err := storage.NewRepository(cfg.Storage)
	if err != nil {
		log.Fatal("Failed to initialize storage", "error", err)
	}
	defer storageRepo.Close()

	metadataRepo := storage.NewMetadataRepository(storageRepo)
	stateRepo := storage.NewStateRepository(storageRepo)

	// Create restore service
	restoreService := restore.NewService(
		kafkaRepo,
		storageRepo,
		metadataRepo,
		stateRepo,
		log,
	)

	// Create and start restore
	restoreConfig := cfg.ToRestoreDomain(*backupID)
	if err := restoreService.CreateRestore(ctx, restoreConfig); err != nil {
		log.Fatal("Failed to create restore", "error", err)
	}

	if err := restoreService.StartRestore(ctx, restoreConfig.ID); err != nil {
		log.Fatal("Failed to start restore", "error", err)
	}

	log.Info("Restore service started successfully")

	// Wait for completion
	<-ctx.Done()
	log.Info("Restore service completed")
}
