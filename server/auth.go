package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ---- User -----------------------------------------------------------------

// User est un utilisateur autorisé (préenregistré au démarrage). La persistance
// est in-memory (choix de projet). L'interface UserStore ci-dessous permet
// d'ajouter un backend PostgreSQL sans toucher au reste du serveur.
type User struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	PasswordHash []byte `json:"-"`
}

// UserStore abstracte l'accès aux comptes. Implémentation par défaut :
// inMemoryUserStore. Un futur store PostgreSQL implémenterait la même interface.
type UserStore interface {
	FindByID(id string) (*User, bool)
	FindByName(name string) (*User, bool)
	Verify(name, password string) (*User, bool)
}

// inMemoryUserStore conserve les comptes en mémoire. Les mots de passe sont
// hachés (bcrypt). Tout disparaît au redémarrage du serveur, conformément au
// cahier des charges (aucune persistance de session/partie).
type inMemoryUserStore struct {
	mu    sync.RWMutex
	byID  map[string]*User
	byNm  map[string]*User
}

func newInMemoryUserStore() *inMemoryUserStore {
	return &inMemoryUserStore{byID: map[string]*User{}, byNm: map[string]*User{}}
}

// add hache le mot de passe en clair puis enregistre l'utilisateur. Doublon
// de nom : écrasement (le dernier gagne).
func (s *inMemoryUserStore) add(id, name, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := &User{ID: id, Name: name, PasswordHash: hash}
	s.byID[id] = u
	s.byNm[strings.ToLower(name)] = u
	return nil
}

func (s *inMemoryUserStore) FindByID(id string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byID[id]
	return u, ok
}

func (s *inMemoryUserStore) FindByName(name string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byNm[strings.ToLower(name)]
	return u, ok
}

// Verify compare le mot de passe fourni au haché stocké (constant-time via
// bcrypt). Retourne l'utilisateur si OK.
func (s *inMemoryUserStore) Verify(name, password string) (*User, bool) {
	u, ok := s.FindByName(name)
	if !ok {
		// On exécute quand même un hachage bidon pour lisser le temps de réponse
		// (évite l'énumération de comptes par chronométrage).
		_ = bcrypt.CompareHashAndPassword(
			[]byte("$2a$10$0123456789012345678901uP9Q5nTqz6wR2l8v3pE2m1n4rWm8uWC"),
			[]byte(password))
		return nil, false
	}
	if bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password)) != nil {
		return nil, false
	}
	return u, true
}

// ---- Sessions -------------------------------------------------------------

// sessionManager distribue des tokens opaques aléatoires associés à un userID.
// In-memory : à la rotation/redémarrage, tous les tokens deviennent invalides
// (les clients doivent se reconnecter et re-s'authentifier).
type sessionManager struct {
	mu      sync.RWMutex
	tokens  map[string]string // token -> userID
}

func newSessionManager() *sessionManager {
	return &sessionManager{tokens: map[string]string{}}
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback faible mais inattendu : on ne doit jamais échouer ici.
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(b)
}

func (sm *sessionManager) create(userID string) string {
	t := newToken()
	sm.mu.Lock()
	sm.tokens[t] = userID
	sm.mu.Unlock()
	return t
}

func (sm *sessionManager) lookup(token string) (string, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	id, ok := sm.tokens[token]
	return id, ok
}

// ---- HTTP /api/login ------------------------------------------------------

// loginRequest est le corps attendu pour POST /api/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse est renvoyé en cas de succès : token de session + identité.
type loginResponse struct {
	Token string `json:"token"`
	ID    string `json:"id"`
	Name  string `json:"name"`
}

// loginHandler valide identifiant + mot de passe et crée une session.
func loginHandler(users UserStore, sessions *sessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		u, ok := users.Verify(req.Username, req.Password)
		if !ok {
			http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
			return
		}
		resp := loginResponse{
			Token: sessions.create(u.ID),
			ID:    u.ID,
			Name:  u.Name,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// ---- Bootstrap ------------------------------------------------------------

// bootstrapUsers charge les utilisateurs depuis la variable d'environnement
// USERS_SEED au format "name:password,name2:password2". Mots de passe hachés
// (bcrypt) puis stockés. Conforme au §10 : comptes préenregistrés, aucune
// création dynamique.
//
// En l'absence de USERS_SEED, on crée deux comptes de démon pour permettre de
// tester l'application immédiatement. Un warning est émis en log.
func bootstrapUsers(store *inMemoryUserStore) {
	raw := strings.TrimSpace(os.Getenv("USERS_SEED"))
	if raw == "" {
		log.Println("ATTENTION: USERS_SEED non défini — création de comptes de démon (alice/secret, bob/secret).")
		raw = "alice:secret,bob:secret"
	}
	for i, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		kv := strings.SplitN(entry, ":", 2)
		if len(kv) != 2 {
			log.Printf("bootstrap: entrée invalide ignorée: %q", entry)
			continue
		}
		name := strings.TrimSpace(kv[0])
		pass := kv[1]
		if name == "" || pass == "" {
			continue
		}
		// L'ID utilisateur est dérivé du nom (stable d'un redémarrage à l'autre
		// pour un même seed) afin que les joueurs gardent une identité cohérente
		// au sein d'une session de développement.
		id := "u-" + name
		if err := store.add(id, name, pass); err != nil {
			log.Printf("bootstrap: impossible d'ajouter %s: %v", name, err)
			continue
		}
		log.Printf("bootstrap: utilisateur %q chargé (index %d)", name, i)
	}
}
