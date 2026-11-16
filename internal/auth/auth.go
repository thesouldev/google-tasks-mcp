package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	tasks "google.golang.org/api/tasks/v1"
)

const signInTimeout = 5 * time.Minute

var ErrSignInRequired = errors.New("not signed in to Google")

func ConfigDir() string {
	if dir := os.Getenv("GOOGLE_TASKS_MCP_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "google-tasks-mcp")
}

func credentialsPath() string { return filepath.Join(ConfigDir(), "credentials.json") }
func TokenPath() string       { return filepath.Join(ConfigDir(), "token.json") }

func oauthConfig() (*oauth2.Config, error) {
	cfg := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Endpoint:     google.Endpoint,
		Scopes:       []string{tasks.TasksScope},
	}
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		return cfg, nil
	}
	raw, err := os.ReadFile(credentialsPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf(
			"no OAuth client found: save your Desktop app client JSON to %s, or set GOOGLE_CLIENT_ID and "+
				"GOOGLE_CLIENT_SECRET. Setup: https://github.com/thesouldev/google-tasks-mcp#oauth-client",
			credentialsPath(),
		)
	}
	if err != nil {
		return nil, err
	}
	var file struct {
		Installed, Web *struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", credentialsPath(), err)
	}
	client := file.Installed
	if client == nil {
		client = file.Web
	}
	if client == nil || client.ClientID == "" || client.ClientSecret == "" {
		return nil, fmt.Errorf("%s has no client_id/client_secret, download it again from Google Cloud Console", credentialsPath())
	}
	cfg.ClientID, cfg.ClientSecret = client.ClientID, client.ClientSecret
	return cfg, nil
}

func loadToken() (*oauth2.Token, error) {
	raw, err := os.ReadFile(TokenPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrSignInRequired
	}
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(raw, &tok); err != nil || tok.RefreshToken == "" {
		return nil, ErrSignInRequired
	}
	return &tok, nil
}

func saveToken(tok *oauth2.Token) error {
	if err := os.MkdirAll(ConfigDir(), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(TokenPath(), raw, 0o600)
}

// Saving refreshed tokens lets the next session start without a refresh round trip.
type persistingSource struct {
	mu   sync.Mutex
	src  oauth2.TokenSource
	last string
}

func (p *persistingSource) Token() (*oauth2.Token, error) {
	tok, err := p.src.Token()
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if tok.AccessToken != p.last {
		p.last = tok.AccessToken
		_ = saveToken(tok)
	}
	return tok, nil
}

func TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	tok, err := loadToken()
	if err != nil {
		return nil, err
	}
	cfg, err := oauthConfig()
	if err != nil {
		return nil, err
	}
	return &persistingSource{src: cfg.TokenSource(ctx, tok), last: tok.AccessToken}, nil
}

func OpenBrowser(url string) {
	if os.Getenv("GOOGLE_TASKS_MCP_NO_BROWSER") != "" {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

type SignIn struct {
	URL string

	done   chan struct{}
	err    error
	once   sync.Once
	cancel chan struct{}
}

func (s *SignIn) Done() <-chan struct{} { return s.done }

func (s *SignIn) Err() error {
	<-s.done
	return s.err
}

func (s *SignIn) Cancel() {
	select {
	case <-s.cancel:
	default:
		close(s.cancel)
	}
}

// Loopback redirect with PKCE is Google's supported flow for desktop apps. PKCE
// stops any other local process that sees the redirect from redeeming the code.
func BeginSignIn() (*SignIn, error) {
	cfg, err := oauthConfig()
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	cfg.RedirectURL = fmt.Sprintf("http://%s", ln.Addr())

	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)
	verifier := oauth2.GenerateVerifier()

	s := &SignIn{
		URL: cfg.AuthCodeURL(state,
			oauth2.AccessTypeOffline,
			oauth2.ApprovalForce,
			oauth2.S256ChallengeOption(verifier),
		),
		done:   make(chan struct{}),
		cancel: make(chan struct{}),
	}

	codes := make(chan string, 1)
	failures := make(chan error, 1)
	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if !q.Has("code") && !q.Has("error") {
				http.NotFound(w, r)
				return
			}
			var fail error
			switch {
			case q.Get("state") != state:
				fail = errors.New("state mismatch, start the sign-in again")
			case q.Has("error"):
				fail = fmt.Errorf("google returned an error: %s", q.Get("error"))
			}
			if fail != nil {
				http.Error(w, fail.Error(), http.StatusBadRequest)
				select {
				case failures <- fail:
				default:
				}
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, "<h1>Signed in to Google Tasks</h1><p>You can close this tab.</p>")
			select {
			case codes <- q.Get("code"):
			default:
			}
		}),
	}
	go func() { _ = srv.Serve(ln) }()

	go func() {
		defer close(s.done)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		}()
		select {
		case code := <-codes:
			s.err = exchange(cfg, code, verifier)
		case s.err = <-failures:
		case <-s.cancel:
			s.err = errors.New("sign-in cancelled")
		case <-time.After(signInTimeout):
			s.err = errors.New("timed out waiting for Google sign-in")
		}
	}()
	return s, nil
}

func exchange(cfg *oauth2.Config, code, verifier string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return err
	}
	if tok.RefreshToken == "" {
		return errors.New("google returned no refresh token: remove the app at " +
			"https://myaccount.google.com/permissions and sign in again")
	}
	return saveToken(tok)
}

func RunAuthFlow() error {
	s, err := BeginSignIn()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Opening your browser to sign in. If it does not open, visit:\n\n%s\n\n", s.URL)
	OpenBrowser(s.URL)
	if err := s.Err(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Signed in. Token saved to %s\n", TokenPath())
	return nil
}

func Logout() error {
	if err := os.Remove(TokenPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Fprintf(os.Stderr, "Removed %s. Revoke access fully at https://myaccount.google.com/permissions\n", TokenPath())
	return nil
}
