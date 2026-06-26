package loopback

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Result struct {
	Code  string
	State string
	Err   error
}

type Server struct {
	RedirectURI string
	server      *http.Server
	results     chan Result
}

func Start(ctx context.Context, address string) (*Server, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("start loopback callback server on %s: %w", address, err)
	}
	results := make(chan Result, 1)
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux}
	redirectURI := "http://" + listener.Addr().String() + "/callback"

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		result := Result{
			Code:  query.Get("code"),
			State: query.Get("state"),
		}
		if query.Get("error") != "" {
			result.Err = fmt.Errorf("consent failed: %s %s", query.Get("error"), query.Get("error_description"))
		} else if result.Code == "" {
			result.Err = errors.New("callback missing authorization code")
		}
		select {
		case results <- result:
		default:
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if result.Err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintln(w, result.Err.Error())
			return
		}
		_, _ = fmt.Fprintln(w, "Tollbit agent authorization received. You can return to the CLI.")
	})

	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case results <- Result{Err: err}:
			default:
			}
		}
	}()

	return &Server{RedirectURI: redirectURI, server: server, results: results}, nil
}

func (s *Server) Wait(ctx context.Context) (Result, error) {
	select {
	case result := <-s.results:
		return result, nil
	case <-ctx.Done():
		return Result{}, fmt.Errorf("timed out waiting for consent callback")
	}
}

func (s *Server) Close() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = s.server.Shutdown(shutdownCtx)
}
