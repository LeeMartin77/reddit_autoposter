package webapp

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"github.com/leemartin77/reddit_autoposter/internal/config"
	"github.com/leemartin77/reddit_autoposter/internal/storage"
	"github.com/rs/zerolog/log"
)

var (
	ErrShutdown error = fmt.Errorf("error shutting down server")
)

func handleStatic(w http.ResponseWriter, r *http.Request) {
	pth := filepath.Join("web", "static", strings.TrimPrefix(r.URL.Path, "/static"))
	file, err := os.Open(pth)
	if err != nil {
		http.Error(w, "No file found", http.StatusNotFound)
		return
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}
	mtype, err := mimetype.DetectReader(file)
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
	w.Header().Set("content-type", mtype.String())
}

func handleFavicon(w http.ResponseWriter, r *http.Request) {
	pth := filepath.Join("web", "static", "favicon.ico")
	file, err := os.Open(pth)
	if err != nil {
		http.Error(w, "No file found", http.StatusNotFound)
		return
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
}

func (app Webapp) Run() error {
	r := gin.Default()

	r.LoadHTMLGlob("web/template/*")

	r.GET("/static/*filepath", func(c *gin.Context) {
		handleStatic(c.Writer, c.Request)
	})

	r.GET("/favicon.ico", func(c *gin.Context) {
		handleFavicon(c.Writer, c.Request)
	})

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"shepard": "commander",
		})
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", "8080"),
		Handler: r,
	}

	log.Info().Msgf("starting on %s", srv.Addr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("critical error starting up")
		}
	}()

	shutdown := make(chan os.Signal, 1)

	signal.Notify(shutdown, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-shutdown

	log.Info().Msg("gracefully shutting down")

	log.Info().Msg("server exiting")

	return nil
}

type Webapp struct {
	cfg  *config.Configuration
	strg storage.Storage
}

type IWebapp interface {
	Run() error
}

func NewWebsite(cfg *config.Configuration) (IWebapp, error) {
	strg, err := storage.NewStorage(cfg.SqliteFile)
	if err != nil {
		return nil, err
	}
	return &Webapp{
		cfg:  cfg,
		strg: strg,
	}, nil
}
