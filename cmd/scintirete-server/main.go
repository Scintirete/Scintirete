// Package main provides the Scintirete server entry point.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"github.com/scintirete/scintirete/internal/config"
	"github.com/scintirete/scintirete/internal/persistence"
	"github.com/scintirete/scintirete/internal/server"
)

var (
	configFile = flag.String("config", "scintirete.toml", "Path to configuration file")
	grpcHost   = flag.String("grpc.host", "", "Override gRPC host from config")
	grpcPort   = flag.Int("grpc.port", 0, "Override gRPC port from config")
	httpPort   = flag.Int("http.port", 0, "Override HTTP port from config")
	dataDir    = flag.String("data-dir", "", "Override data directory from config")
	logLevel   = flag.String("log.level", "", "Override log level from config")
	version    = flag.Bool("version", false, "Show version information")
)

func main() {
	flag.Parse()

	if *version {
		fmt.Println("Scintirete Server v1.0.0")
		fmt.Println("A lightweight vector database with HNSW indexing")
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Apply command line overrides
	if *grpcHost != "" {
		cfg.Server.GRPCHost = *grpcHost
	}
	if *grpcPort != 0 {
		cfg.Server.GRPCPort = *grpcPort
	}
	if *httpPort != 0 {
		cfg.Server.HTTPPort = *httpPort
	}
	if *dataDir != "" {
		cfg.Persistence.DataDir = *dataDir
	}
	if *logLevel != "" {
		cfg.Log.Level = *logLevel
	}

	// Note: Configuration validation can be added here if needed

	// Create server configuration
	serverConfig := server.Config{
		Passwords: cfg.Server.Passwords,
		PersistenceConfig: persistence.Config{
			DataDir:         cfg.Persistence.DataDir,
			RDBFilename:     cfg.Persistence.RDBFilename,
			AOFFilename:     cfg.Persistence.AOFFilename,
			AOFSyncStrategy: cfg.Persistence.AOFSyncStrategy,
			RDBInterval:     5 * time.Minute,
			AOFRewriteSize:  64 * 1024 * 1024,
			BackupRetention: 7,
		},
		EnableMetrics:  cfg.Observability.MetricsEnabled,
		EnableAuditLog: cfg.Log.EnableAuditLog,
	}

	// Create gRPC server
	grpcServer, err := server.NewGRPCServer(serverConfig)
	if err != nil {
		log.Fatalf("Failed to create gRPC server: %v", err)
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := grpcServer.Start(ctx); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Start gRPC server in a goroutine
	grpcAddr := fmt.Sprintf("%s:%d", cfg.Server.GRPCHost, cfg.Server.GRPCPort)
	go func() {
		log.Printf("Starting gRPC server on %s", grpcAddr)

		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatalf("Failed to listen on %s: %v", grpcAddr, err)
		}

		s := grpc.NewServer()
		pb.RegisterScintireteServiceServer(s, grpcServer)

		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Start HTTP gateway server
	httpServer := server.NewHTTPServer(grpcServer)
	httpAddr := cfg.GetHTTPAddress()

	go func() {
		log.Printf("Starting HTTP server on %s", httpAddr)
		if err := http.ListenAndServe(httpAddr, httpServer); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	log.Printf("Scintirete server started successfully")
	log.Printf("gRPC endpoint: %s", grpcAddr)
	log.Printf("HTTP endpoint: %s", httpAddr)

	// Wait for shutdown signal
	<-shutdown
	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := grpcServer.Stop(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	log.Println("Server stopped")
}
