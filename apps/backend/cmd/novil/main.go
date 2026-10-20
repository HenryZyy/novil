// Command novil runs the novil media server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"novil/backend/internal/api"
	"novil/backend/internal/auth"
	"novil/backend/internal/config"
	"novil/backend/internal/cover"
	"novil/backend/internal/db"
	"novil/backend/internal/isbn"
	"novil/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("novil stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadEnvFile(".env"); err != nil {
		return fmt.Errorf("load .env: %w", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// It holds the database and the token key: only for the server.
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return err
	}
	database, err := db.Open(filepath.Join(cfg.DataDir, "novil.db"))
	if err != nil {
		return err
	}
	defer database.Close()
	st := store.New(database)

	secret, err := auth.LoadSecret(cfg.JWTSecret, filepath.Join(cfg.DataDir, "jwt.key"))
	if err != nil {
		return fmt.Errorf("NOVIL_JWT_SECRET: %w", err)
	}
	if err := ensureAdmin(st, cfg); err != nil {
		return err
	}
	proxies, err := parsePrefixes(cfg.TrustedProxies)
	if err != nil {
		return fmt.Errorf("NOVIL_TRUSTED_PROXIES: %w", err)
	}

	covers := cover.NewGenerator()
	defer covers.Close()
	handler := api.New(st, auth.NewTokens(secret, tokenTTL), api.Options{
		Covers:         covers,
		WebDir:         cfg.WebDir,
		MaxUploadBytes: cfg.MaxUploadBytes,
		CORSOrigins:    cfg.CORSOrigins,
		TrustedProxies: proxies,
		LookupISBN:     isbn.Online(cfg.GoogleBooksKey),
	})
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// Uploads can be large and slow, so bodies have no overall limit;
		// JSON handlers set their own deadline (see api.decodeJSON).
		IdleTimeout: 2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		slog.Info("novil listening", "addr", cfg.Addr, "data", cfg.DataDir)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return nil
}

// parsePrefixes parses IPs and CIDR ranges. A single IP becomes a range
// containing only itself.
func parsePrefixes(values []string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, v := range values {
		if p, err := netip.ParsePrefix(v); err == nil {
			out = append(out, p.Masked())
			continue
		}
		addr, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("%q is not an IP or CIDR range", v)
		}
		out = append(out, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
	}
	return out, nil
}

// tokenTTL is how long a login lasts before the user has to log in again.
const tokenTTL = 7 * 24 * time.Hour

// ensureAdmin creates the first admin from NOVIL_ADMIN_USER and
// NOVIL_ADMIN_PASSWORD if there are no users yet, since only admins can
// create accounts. Like every new user, they must change the password after
// logging in.
func ensureAdmin(st *store.Store, cfg config.Config) error {
	n, err := st.CountUsers()
	if err != nil || n > 0 {
		return err
	}
	if cfg.AdminUser == "" || cfg.AdminPassword == "" {
		return errors.New("there are no users yet: set NOVIL_ADMIN_USER and NOVIL_ADMIN_PASSWORD to create the first admin")
	}
	if err := auth.ValidateUsername(cfg.AdminUser); err != nil {
		return fmt.Errorf("NOVIL_ADMIN_USER: %w", err)
	}
	if err := auth.ValidatePassword(cfg.AdminPassword); err != nil {
		return fmt.Errorf("NOVIL_ADMIN_PASSWORD: %w", err)
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	admin, err := st.CreateUser(cfg.AdminUser, hash, store.RoleAdmin, true)
	if err != nil {
		return err
	}
	slog.Info("created first admin", "username", admin.Username)
	return nil
}
