package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/quantica-technologies/kafka-backup-operator/internal/app/backup"
	"github.com/quantica-technologies/kafka-backup-operator/internal/config"
	"github.com/quantica-technologies/kafka-backup-operator/internal/infrastructure/kafka"
	"github.com/quantica-technologies/kafka-backup-operator/internal/infrastructure/storage"
	"github.com/quantica-technologies/kafka-backup-operator/pkg/logger"
)

const version = "1.0.0"

func main() {
	var (
		configFile  = flag.String("config", "config.yaml", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Kafka Backup v%s\n", version)
		return
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

	// Create backup service
	backupService := backup.NewService(
		kafkaRepo,
		storageRepo,
		metadataRepo,
		stateRepo,
		log,
	)

	// Create and start backup
	backupConfig := cfg.ToBackupDomain()
	if err := backupService.CreateBackup(ctx, backupConfig); err != nil {
		log.Fatal("Failed to create backup", "error", err)
	}

	if err := backupService.StartBackup(ctx, backupConfig.ID); err != nil {
		log.Fatal("Failed to start backup", "error", err)
	}

	log.Info("Backup service started successfully")

	// Wait for context cancellation
	<-ctx.Done()
	log.Info("Backup service stopped")
}
