package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	chat "appartament_building_chat/internal/chat"
	"appartament_building_chat/internal/config"

	ssogrpc "appartament_building_chat/internal/clients/sso/grpc"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type Server struct {
	router *mux.Router
	logger *slog.Logger
	sso    *ssogrpc.Client
	appID  int64
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

type key int

const (
	ctxKeyUser key = iota
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
var appSecret string

func newServer(config *config.Config, appCfg *config.AppConfig, hub *chat.Hub, sso *ssogrpc.Client) *Server {
	s := &Server{
		router: mux.NewRouter(),
		logger: func() *slog.Logger {
			var log *slog.Logger
			switch config.LogLevel {
			case "error":
				log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			case "info":
				log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
			case "warn":
				log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
			case "debug":
				log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			}
			return log
		}(),
		sso:   sso,
		appID: appCfg.AppID,
	}
	appSecret = appCfg.AppSecret
	s.configureRouter(hub)
	return s
}

func (s *Server) configureRouter(hub *chat.Hub) {
	s.router.Use(handlers.CORS(handlers.AllowedOrigins([]string{"https://localhost:8080"}), // тут воровская звезда
		handlers.AllowedMethods([]string{"GET", "POST", "OPTIONS", "HEAD", "PUT"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Accept", "Origin", "X-Request-ID", "Allow", "Set-Cookie", "Cookie"}),
		handlers.AllowCredentials(),
	))
	s.router.HandleFunc("/login", s.Login()).Methods(http.MethodOptions, http.MethodPost)
	s.router.HandleFunc("/register", s.Register()).Methods(http.MethodOptions, http.MethodPost)
	s.router.HandleFunc("/ws", s.ServeWs(hub)).Methods(http.MethodOptions, http.MethodGet)
	private := s.router.PathPrefix("/private").Subrouter()
	private.Use(s.authenticateUser)
}

func (s *Server) authenticateUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			return []byte(appSecret), nil
		})
		if err != nil {
			s.error(w, http.StatusInternalServerError, err)
		}
		claims, ok := parsedToken.Claims.(jwt.MapClaims)
		if !ok {
			s.error(w, http.StatusUnauthorized, nil)
		}
		uuID, ok := claims["uid"].(string)
		if !ok {
			s.error(w, http.StatusInternalServerError, nil)
		}
		parsedUUID, err := uuid.Parse(uuID)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err)
		}
		userAcc, err := s.sso.Find(r.Context(), parsedUUID)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyUser, userAcc)))
	})
}

func (s *Server) sendMessageAuth() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			return []byte(appSecret), nil
		})
		if err != nil {
			s.error(w, http.StatusInternalServerError, err)
		}
		claims, ok := parsedToken.Claims.(jwt.MapClaims)
		if !ok {
			s.error(w, http.StatusUnauthorized, nil)
		}
		uuID, ok := claims["uid"].(string)
		if !ok {
			s.error(w, http.StatusInternalServerError, nil)
		}
		parsedUUID, err := uuid.Parse(uuID)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err)
		}
		_, err = s.sso.Find(r.Context(), parsedUUID)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err)
		}
	})
}

func (s *Server) Login() http.HandlerFunc {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		req := &request{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			s.error(w, http.StatusBadRequest, err)
			return
		}
		token, err := s.sso.Login(r.Context(), req.Email, req.Password, s.appID)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Authorization", token)
		s.respond(w, http.StatusOK, nil)
	}
}

func (s *Server) Register() http.HandlerFunc {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Username string `json:"username"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		req := &request{}
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			s.error(w, http.StatusBadRequest, err)
			return
		}
		s.logger.Info(fmt.Sprintf("Request in HUC: %s, %s, %s", req.Username, req.Email, req.Password))
		userID, err := s.sso.Register(r.Context(), req.Email, req.Password)
		if err != nil {
			s.error(w, http.StatusInternalServerError, err)
			return
		}
		s.respond(w, http.StatusCreated, userID)
	}
}

// serveWs handles websocket requests from the peer.
func (s *Server) ServeWs(hub *chat.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		client := chat.New(hub, conn, make(chan chat.Message))
		client.ClientRegister()
		// Allow collection of memory referenced by the caller by doing all work in
		// new goroutines.

		messageRead := make(chan struct{})
		go func(messageRead chan struct{}) {
			for {
				<-messageRead
				s.sendMessageAuth()
				messageRead <- struct{}{}
			}
		}(messageRead)
		go client.WritePump()
		go client.ReadPump(messageRead)

	}
}

func (s *Server) error(w http.ResponseWriter, code int, err error) {
	slog.Error(fmt.Sprintf("error: %s", err.Error()))
	s.respond(w, code, map[string]string{"error": err.Error()})
}
func (s *Server) respond(w http.ResponseWriter, code int, data ...interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}
