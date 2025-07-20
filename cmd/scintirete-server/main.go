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
	"google.golang.org/grpc/reflection"

	pb "github.com/scintirete/scintirete/gen/go/scintirete/v1"
	"github.com/scintirete/scintirete/internal/config"
	"github.com/scintirete/scintirete/internal/embedding"
	"github.com/scintirete/scintirete/internal/persistence"
	"github.com/scintirete/scintirete/internal/server"
	grpcserver "github.com/scintirete/scintirete/internal/server/grpc"
	httpserver "github.com/scintirete/scintirete/internal/server/http"
)

var (
	configFile = flag.String("config", "configs/scintirete.toml", "Path to configuration file")
	logLevel   = flag.String("log-level", "", "Log level (debug, info, warn, error)")
	help       = flag.Bool("help", false, "Show help message")
)

func main() {
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	// Print banner
	printBanner()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override log level from command line
	if *logLevel != "" {
		cfg.Log.Level = *logLevel
	}

	// Note: Configuration validation can be added here if needed

	// Create server configuration
	serverConfig := server.ServerConfig{
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
		EmbeddingConfig: embedding.Config{
			BaseURL: cfg.Embedding.BaseURL,
			APIKey:  cfg.Embedding.APIKey,
			Timeout: 30 * time.Second, // Use default timeout
		},
		EnableMetrics:  cfg.Observability.MetricsEnabled,
		EnableAuditLog: cfg.Log.EnableAuditLog,
	}

	// Create gRPC server
	grpcServer, err := grpcserver.NewServer(serverConfig)
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
		reflection.Register(s)

		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Start HTTP gateway server
	httpServer := httpserver.NewServer(grpcServer)
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

	// Graceful shutdown
	cancel()
	if err := grpcServer.Stop(ctx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	log.Println("Server shutdown complete")
}

func printBanner() {
	banner := `
   _____ _____ _____ _   _ _______ _____ _____  ______ _______ ______ 
  / ____/ ____|_   _| \ | |__   __|_   _|  __ \|  ____|__   __|  ____|
 | (___| |      | | |  \| |  | |    | | | |__) | |__     | |  | |__   
  \___ \ |      | | | . ' |  | |    | | |  _  /|  __|    | |  |  __|  
  ____) | |____ _| |_| |\  |  | |   _| |_| | \ \| |____   | |  | |____ 
 |_____/ \_____|_____|_| \_|  |_|  |_____|_|  \_\______|  |_|  |______|
                                                                      
`
	fmt.Println(banner)
	fmt.Println("High-Performance Vector Database")
	fmt.Println()
}
