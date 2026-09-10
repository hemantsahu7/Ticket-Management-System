package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

const (
	statusOpen       = "open"
	statusInProgress = "in_progress"
	statusClosed     = "closed"
)

type app struct {
	db        *sql.DB
	jwtSecret []byte
}

type user struct {
	ID           int
	Email        string
	PasswordHash string
}

type ticket struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	UserID      int       `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type credentialsInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type createTicketInput struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

type updateStatusInput struct {
	Status string `json:"status" binding:"required,oneof=open in_progress closed"`
}

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("could not load .env: %v", err)
	}

	db, err := openDatabase()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	secret := requiredEnv("JWT_SECRET")

	a := &app{db: db, jwtSecret: []byte(secret)}
	router := gin.Default()
	router.Use(corsMiddleware())

	router.GET("/health", a.health)
	router.POST("/auth/register", a.register)
	router.POST("/auth/login", a.login)

	tickets := router.Group("/tickets", a.authMiddleware())
	{
		tickets.POST("", a.createTicket)
		tickets.GET("", a.listTickets)
		tickets.GET("/:id", a.getTicket)
		tickets.PATCH("/:id/status", a.updateTicketStatus)
	}

	port := requiredEnv("PORT")

	log.Printf("ticket system listening on :%s", port)
	log.Fatal(router.Run(":" + port))
}

func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		log.Fatalf("%s must be set in the .env file or environment", name)
	}
	return value
}

func openDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	// An in-memory SQLite database exists per connection, so use one connection.
	db.SetMaxOpenConns(1)
	if err := createSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (a *app) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (a *app) register(c *gin.Context) {
	var input credentialsInput
	if !bindJSON(c, &input) {
		return
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not create user")
		return
	}

	result, err := a.db.Exec(`INSERT INTO users (email, password_hash) VALUES (?, ?)`, email, string(passwordHash))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			writeError(c, http.StatusConflict, "email is already registered")
		} else {
			writeError(c, http.StatusInternalServerError, "could not create user")
		}
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{"id": id, "email": email})
}

func (a *app) login(c *gin.Context) {
	var input credentialsInput
	if !bindJSON(c, &input) {
		return
	}

	var u user
	email := strings.ToLower(strings.TrimSpace(input.Email))
	err := a.db.QueryRow(`SELECT id, email, password_hash FROM users WHERE email = ?`, email).Scan(&u.ID, &u.Email, &u.PasswordHash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(input.Password)) != nil {
		writeError(c, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := a.createToken(u.ID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not create token")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (a *app) createTicket(c *gin.Context) {
	var input createTicketInput
	if !bindJSON(c, &input) {
		return
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		writeError(c, http.StatusBadRequest, "title is required")
		return
	}

	now := time.Now().UTC()
	userID := c.GetInt("userID")
	result, err := a.db.Exec(
		`INSERT INTO tickets (title, description, status, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		title, strings.TrimSpace(input.Description), statusOpen, userID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not create ticket")
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, ticket{ID: int(id), Title: title, Description: strings.TrimSpace(input.Description), Status: statusOpen, UserID: userID, CreatedAt: now, UpdatedAt: now})
}

func (a *app) listTickets(c *gin.Context) {
	rows, err := a.db.Query(`SELECT id, title, description, status, user_id, created_at, updated_at FROM tickets WHERE user_id = ? ORDER BY id`, c.GetInt("userID"))
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not list tickets")
		return
	}
	defer rows.Close()

	tickets := make([]ticket, 0)
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "could not read tickets")
			return
		}
		tickets = append(tickets, t)
	}
	c.JSON(http.StatusOK, tickets)
}

func (a *app) getTicket(c *gin.Context) {
	t, ok := a.ownedTicket(c)
	if ok {
		c.JSON(http.StatusOK, t)
	}
}

func (a *app) updateTicketStatus(c *gin.Context) {
	var input updateStatusInput
	if !bindJSON(c, &input) {
		return
	}

	t, ok := a.ownedTicket(c)
	if !ok {
		return
	}
	if !validTransition(t.Status, input.Status) {
		writeError(c, http.StatusBadRequest, "invalid status transition")
		return
	}

	t.Status = input.Status
	t.UpdatedAt = time.Now().UTC()
	_, err := a.db.Exec(`UPDATE tickets SET status = ?, updated_at = ? WHERE id = ?`, t.Status, t.UpdatedAt.Format(time.RFC3339Nano), t.ID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not update ticket")
		return
	}
	c.JSON(http.StatusOK, t)
}

func (a *app) ownedTicket(c *gin.Context) (ticket, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		writeError(c, http.StatusNotFound, "ticket not found")
		return ticket{}, false
	}

	t, err := a.ticketByID(id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && t.UserID != c.GetInt("userID")) {
		writeError(c, http.StatusNotFound, "ticket not found")
		return ticket{}, false
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not read ticket")
		return ticket{}, false
	}
	return t, true
}

func validTransition(from, to string) bool {
	return (from == statusOpen && to == statusInProgress) || (from == statusInProgress && to == statusClosed)
}

func (a *app) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(c, http.StatusUnauthorized, "missing bearer token")
			c.Abort()
			return
		}

		userID, err := a.tokenUserID(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			writeError(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}
		c.Set("userID", userID)
		c.Next()
	}
}

func (a *app) createToken(userID int) (string, error) {
	claims := jwt.MapClaims{
		"sub": strconv.Itoa(userID),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.jwtSecret)
}

func (a *app) tokenUserID(tokenString string) (int, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return a.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("invalid claims")
	}
	subject, ok := claims["sub"].(string)
	if !ok {
		return 0, errors.New("invalid subject")
	}
	userID, err := strconv.Atoi(subject)
	if err != nil || userID < 1 {
		return 0, errors.New("invalid subject")
	}
	return userID, nil
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}
