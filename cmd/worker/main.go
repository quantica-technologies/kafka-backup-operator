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

func main() {
	var (
		configFile = flag.String("config", "config.yaml", "Path to configuration file")
		mode       = flag.String("mode", "backup", "Mode: backup or restore")
		workerID   = flag.Int("worker-id", 0, "Worker ID")
		backupID   = flag.String("backup-id", "", "Backup/Restore ID")
	)
	flag.Parse()

	if *backupID == "" {
		fmt.Fprintf(os.Stderr, "Backup/Restore ID is required\n")
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
	}).WithFields(map[string]interface{}{
		"workerID": *workerID,
		"mode":     *mode,
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

	switch *mode {
	case "backup":
		// Retrieve backup configuration
		backupService := backup.NewService(kafkaRepo, storageRepo, metadataRepo, stateRepo, log)
		backupObj, err := backupService.GetBackup(ctx, *backupID)
		if err != nil {
			log.Fatal("Failed to get backup", "error", err)
		}

		// Get topic assignment for this worker
		allTopics, _ := kafkaRepo.CreateAdmin(ctx, backupObj.SourceCluster)
		// ... topic distribution logic

		log.Info("Worker started successfully")
		<-ctx.Done()

	case "restore":
		// Similar for restore worker
		log.Info("Restore worker started")
		<-ctx.Done()

	default:
		log.Fatal("Invalid mode", "mode", *mode)
	}

	log.Info("Worker stopped")
}
