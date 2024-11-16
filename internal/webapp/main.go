package webapp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
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
		code := uuid.NewString()

		app.codeStore = append(app.codeStore, code)

		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"signinLink": fmt.Sprintf("https://www.reddit.com/api/v1/authorize?client_id=%s&response_type=code&state=%s&redirect_uri=%s&duration=permanent&scope=submit,identity",
				app.cfg.AuthRedditAppId,
				code,
				app.cfg.AuthRedditRedirectUrl,
			),
		})
	})
	r.GET("/auth/callback", func(c *gin.Context) {
		e, hasError := c.GetQuery("error")
		if hasError {
			c.HTML(http.StatusBadRequest, "error.tmpl", gin.H{"errorCode": e, "errorMessage": "Unable to authenticate"})
			return
		}

		code, hasCode := c.GetQuery("code")
		fmt.Println(code)
		if !hasCode {
			c.HTML(http.StatusBadRequest, "error.tmpl", gin.H{"errorCode": "400", "errorMessage": "Missing code"})
			return
		}
		state, hasState := c.GetQuery("state")
		if !hasState {
			c.HTML(http.StatusBadRequest, "error.tmpl", gin.H{"errorCode": "400", "errorMessage": "Missing state"})
			return
		}

		if !slices.Contains(app.codeStore, state) {
			c.HTML(http.StatusBadRequest, "error.tmpl", gin.H{"errorCode": "400", "errorMessage": "Unrecognised state"})
			return
		}

		data := map[string]string{"grant_type": "authorization_code", "code": code, "redirect_uri": app.cfg.AuthRedditRedirectUrl}
		form := url.Values{}
		for k, v := range data {
			form.Set(k, v)
		}
		client := &http.Client{}
		req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", bytes.NewBufferString(form.Encode()))
		if err != nil {
			log.Error().Err(err).Msg("error creating request")
			c.HTML(http.StatusInternalServerError, "error.tmpl", gin.H{"errorCode": "500", "errorMessage": "Unexpected error"})
		}
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(app.cfg.AuthRedditAppId+":"+app.cfg.AuthRedditAppSecret))))

		resp, err := client.Do(req)
		if err != nil {
			log.Error().Err(err).Msg("error sending request to reddit")
			c.HTML(http.StatusInternalServerError, "error.tmpl", gin.H{"errorCode": "500", "errorMessage": "Unexpected error"})
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			log.Error().Msg("fuck you I guess")
			c.HTML(http.StatusInternalServerError, "error.tmpl", gin.H{"errorCode": "666", "errorMessage": "Reddit API limits are dumb"})
			return
		}
		if resp.StatusCode != http.StatusOK {
			rawBody, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Error().Err(err).Msg("error even getting body")
			}
			log.Error().Any("raw response", string(rawBody)).Msg("Raw error response from reddit")

			c.HTML(http.StatusInternalServerError, "error.tmpl", gin.H{"errorCode": "500", "errorMessage": "Unexpected error"})
			return
		}

		var result TokenResponse

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Error().Err(err).Msg("error decoding token")
			c.HTML(http.StatusInternalServerError, "error.tmpl", gin.H{"errorCode": "500", "errorMessage": "Unexpected error"})
			return
		}

		// this is just for testing
		jsonString, _ := json.Marshal(result)

		// TODO:
		// lookup associated username (?)
		// save access token
		// set a cookie
		// redirect to /home

		c.HTML(http.StatusOK, "successful_login.tmpl", gin.H{"message": string(jsonString)})
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
	cfg       *config.Configuration
	strg      storage.Storage
	codeStore []string
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
		cfg:       cfg,
		strg:      strg,
		codeStore: []string{},
	}, nil
}
