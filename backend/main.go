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

// These are the three possible stages of a ticket.
// A ticket starts as "open", then can move to "in_progress",
// and finally to "closed".
const (
	statusOpen       = "open"
	statusInProgress = "in_progress"
	statusClosed     = "closed"
)

// The app structure keeps the resources that are shared
// throughout the application, such as the database connection
// and the secret key used to create and verify login tokens.
type app struct {
	db        *sql.DB
	jwtSecret []byte
}

// Represents a user stored in the database.
type user struct {
	ID           int
	Email        string
	PasswordHash string
}

// Represents a support ticket.
// The JSON names below define how ticket information is sent
// to and received from the frontend.
type ticket struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	UserID      int       `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Represents the information required when a user registers
// or logs in. The validation rules make sure an email and
// a password of at least 8 characters are provided.
type credentialsInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// Represents the information needed to create a new ticket.
// Only the title is mandatory; the description is optional.
type createTicketInput struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

// Represents the new status provided when a user wants
// to move a ticket to another stage.
type updateStatusInput struct {
	Status string `json:"status" binding:"required,oneof=open in_progress closed"`
}

func main() {
	// Load configuration values such as the database settings,
	// port, and JWT secret from the local .env file.
	// If the file does not exist, the application can still continue
	// because these values may have been provided by the environment.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("could not load .env: %v", err)
	}

	// Create and prepare the database used by the application.
	db, err := openDatabase()
	if err != nil {
		log.Fatal(err)
	}
	// Close the database connection when the application stops.
	defer db.Close()

	// Get the secret used to securely create and verify login tokens.
	secret := requiredEnv("JWT_SECRET")

	// Create the main application object containing the database
	// and security information that the different handlers will use.
	a := &app{db: db, jwtSecret: []byte(secret)}

	// Create the web server and enable Gin's default logging
	// and recovery features.
	router := gin.Default()

	// Allow the frontend to communicate with this backend
	// even when they are running on different ports.
	router.Use(corsMiddleware())

	// Public endpoints that do not require the user to be logged in.
	router.GET("/health", a.health)
	router.POST("/auth/register", a.register)
	router.POST("/auth/login", a.login)

	// All ticket-related endpoints require a valid login token.
	tickets := router.Group("/tickets", a.authMiddleware())
	{
		// Create a new ticket.
		tickets.POST("", a.createTicket)

		// Get all tickets belonging to the logged-in user.
		tickets.GET("", a.listTickets)

		// Get one specific ticket.
		tickets.GET("/:id", a.getTicket)

		// Change the status of a specific ticket.
		tickets.PATCH("/:id/status", a.updateTicketStatus)
	}

	// Read the port on which the web server should listen.
	port := requiredEnv("PORT")

	log.Printf("ticket system listening on :%s", port)

	// Start the web server and keep it running.
	log.Fatal(router.Run(":" + port))
}

// Reads a required setting from the environment.
// If the setting is missing, the application stops and explains
// which setting needs to be provided.
func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		log.Fatalf("%s must be set in the .env file or environment", name)
	}
	return value
}

// Opens the SQLite database and prepares its tables.
func openDatabase() (*sql.DB, error) {
	// Create an SQLite database that exists only while this
	// application is running.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}

	// An in-memory SQLite database exists per connection, so use one connection.
	db.SetMaxOpenConns(1)

	// Create the required tables and indexes.
	if err := createSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// Configures the rules that allow the frontend to communicate
// with the backend, including which types of requests are allowed.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")

		// Browsers send an OPTIONS request before some requests
		// to check whether the server allows the requested operation.
		// This responds to that check without performing another action.
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}

		c.Next()
	}
}

// Simple endpoint used to check whether the backend is running correctly.
func (a *app) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Creates a new user account.
func (a *app) register(c *gin.Context) {
	var input credentialsInput

	// Read and validate the email and password sent by the user.
	if !bindJSON(c, &input) {
		return
	}

	// Normalize the email so that different capitalization
	// does not result in multiple accounts for the same email.
	email := strings.ToLower(strings.TrimSpace(input.Email))

	// Convert the password into a secure hash before storing it.
	// The original password is never stored in the database.
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not create user")
		return
	}

	// Save the new user's email and password hash in the database.
	result, err := a.db.Exec(`INSERT INTO users (email, password_hash) VALUES (?, ?)`, email, string(passwordHash))
	if err != nil {
		// The database prevents duplicate email addresses.
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			writeError(c, http.StatusConflict, "email is already registered")
		} else {
			writeError(c, http.StatusInternalServerError, "could not create user")
		}
		return
	}

	// Return the ID assigned to the newly created user.
	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{"id": id, "email": email})
}

// Logs an existing user into the system.
func (a *app) login(c *gin.Context) {
	var input credentialsInput

	// Read and validate the login information.
	if !bindJSON(c, &input) {
		return
	}

	var u user

	// Normalize the email in the same way as registration.
	email := strings.ToLower(strings.TrimSpace(input.Email))

	// Find the user with the provided email address.
	err := a.db.QueryRow(`SELECT id, email, password_hash FROM users WHERE email = ?`, email).Scan(&u.ID, &u.Email, &u.PasswordHash)

	// Check both that the user exists and that the supplied password
	// matches the securely stored password hash.
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(input.Password)) != nil {
		writeError(c, http.StatusUnauthorized, "invalid email or password")
		return
	}

	// Create a login token that the frontend can use
	// when making requests to protected ticket endpoints.
	token, err := a.createToken(u.ID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not create token")
		return
	}

	// Send the login token back to the frontend.
	c.JSON(http.StatusOK, gin.H{"token": token})
}

// Creates a new ticket for the currently logged-in user.
func (a *app) createTicket(c *gin.Context) {
	var input createTicketInput

	// Read and validate the ticket information sent by the user.
	if !bindJSON(c, &input) {
		return
	}

	// Remove unnecessary spaces from the title.
	title := strings.TrimSpace(input.Title)

	// A ticket cannot be created without a title.
	if title == "" {
		writeError(c, http.StatusBadRequest, "title is required")
		return
	}

	// Use the current UTC time for both creation and update timestamps.
	now := time.Now().UTC()

	// Get the ID of the user who is creating the ticket.
	// This ID was stored earlier by the authentication middleware.
	userID := c.GetInt("userID")

	// Store the new ticket in the database.
	// Every new ticket starts with an "open" status.
	result, err := a.db.Exec(
		`INSERT INTO tickets (title, description, status, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		title, strings.TrimSpace(input.Description), statusOpen, userID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not create ticket")
		return
	}

	// Get the ID automatically assigned to the new ticket
	// and return the complete ticket information to the frontend.
	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, ticket{ID: int(id), Title: title, Description: strings.TrimSpace(input.Description), Status: statusOpen, UserID: userID, CreatedAt: now, UpdatedAt: now})
}

// Returns all tickets belonging to the currently logged-in user.
func (a *app) listTickets(c *gin.Context) {
	// Ask the database for tickets owned by the current user.
	// Tickets are returned in their ID order.
	rows, err := a.db.Query(`SELECT id, title, description, status, user_id, created_at, updated_at FROM tickets WHERE user_id = ? ORDER BY id`, c.GetInt("userID"))
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not list tickets")
		return
	}

	// Make sure the database result is closed after we finish using it.
	defer rows.Close()

	tickets := make([]ticket, 0)

	// Process each ticket returned by the database.
	for rows.Next() {
		// Convert the database row into a ticket object.
		t, err := scanTicket(rows)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "could not read tickets")
			return
		}

		tickets = append(tickets, t)
	}

	// Send the user's tickets back to the frontend.
	c.JSON(http.StatusOK, tickets)
}

// Returns one specific ticket.
// The ticket must belong to the currently logged-in user.
func (a *app) getTicket(c *gin.Context) {
	t, ok := a.ownedTicket(c)
	if ok {
		c.JSON(http.StatusOK, t)
	}
}

// Changes the status of an existing ticket.
func (a *app) updateTicketStatus(c *gin.Context) {
	var input updateStatusInput

	// Read and validate the requested new status.
	if !bindJSON(c, &input) {
		return
	}

	// Make sure the requested ticket exists and belongs to
	// the currently logged-in user.
	t, ok := a.ownedTicket(c)
	if !ok {
		return
	}

	// Make sure the ticket is being moved through the allowed
	// sequence of statuses.
	if !validTransition(t.Status, input.Status) {
		writeError(c, http.StatusBadRequest, "invalid status transition")
		return
	}

	// Update the status and record the time of the change.
	t.Status = input.Status
	t.UpdatedAt = time.Now().UTC()

	// Save the updated ticket information in the database.
	_, err := a.db.Exec(`UPDATE tickets SET status = ?, updated_at = ? WHERE id = ?`, t.Status, t.UpdatedAt.Format(time.RFC3339Nano), t.ID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not update ticket")
		return
	}

	// Return the updated ticket to the frontend.
	c.JSON(http.StatusOK, t)
}

// Finds a ticket and confirms that it belongs to the logged-in user.
// This prevents one user from viewing or modifying another user's ticket.
func (a *app) ownedTicket(c *gin.Context) (ticket, bool) {
	// Convert the ticket ID from the URL from text into a number.
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		writeError(c, http.StatusNotFound, "ticket not found")
		return ticket{}, false
	}

	// Find the requested ticket in the database.
	t, err := a.ticketByID(id)

	// If the ticket does not exist, or belongs to another user,
	// report it as not found instead of exposing another user's data.
	if errors.Is(err, sql.ErrNoRows) || (err == nil && t.UserID != c.GetInt("userID")) {
		writeError(c, http.StatusNotFound, "ticket not found")
		return ticket{}, false
	}

	// Handle unexpected database errors.
	if err != nil {
		writeError(c, http.StatusInternalServerError, "could not read ticket")
		return ticket{}, false
	}

	return t, true
}

// Defines the only valid order in which a ticket can change status:
// open -> in_progress -> closed.
func validTransition(from, to string) bool {
	return (from == statusOpen && to == statusInProgress) || (from == statusInProgress && to == statusClosed)
}

// Checks whether a request contains a valid login token.
// This middleware runs before the protected ticket endpoints.
func (a *app) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read the login token from the Authorization header.
		header := c.GetHeader("Authorization")

		// The expected format is: "Bearer <token>".
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(c, http.StatusUnauthorized, "missing bearer token")
			c.Abort()
			return
		}

		// Verify the token and find out which user it belongs to.
		userID, err := a.tokenUserID(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			writeError(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		// Store the user's ID so the ticket handlers know
		// which user is making the request.
		c.Set("userID", userID)
		c.Next()
	}
}

// Creates a signed login token for a user.
// The token expires after 24 hours.
func (a *app) createToken(userID int) (string, error) {
	// Store the user's ID and the token expiration time
	// inside the token.
	claims := jwt.MapClaims{
		"sub": strconv.Itoa(userID),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}

	// Sign the token with the application's secret key.
	// This allows the server to detect if someone has modified it.
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.jwtSecret)
}

// Checks a login token and returns the ID of the user it belongs to.
func (a *app) tokenUserID(tokenString string) (int, error) {
	// Read and verify the token using the application's secret key.
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		// Only accept tokens created using the expected signing method.
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}

		return a.jwtSecret, nil
	})

	// Reject tokens that are invalid or have expired.
	if err != nil || !token.Valid {
		return 0, errors.New("invalid token")
	}

	// Read the information stored inside the verified token.
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("invalid claims")
	}

	// Get the user ID stored in the token.
	subject, ok := claims["sub"].(string)
	if !ok {
		return 0, errors.New("invalid subject")
	}

	// Convert the user ID from text into a number.
	userID, err := strconv.Atoi(subject)
	if err != nil || userID < 1 {
		return 0, errors.New("invalid subject")
	}

	return userID, nil
}

// Reads JSON data sent by the frontend and checks that it
// follows the validation rules defined by the corresponding input structure.
func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return false
	}

	return true
}

// Sends a consistent error response back to the frontend.
func writeError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}
