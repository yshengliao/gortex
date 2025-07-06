// Package services provides service interfaces for business logic
package services

import (
	"context"
	"time"
)

// Service is the base interface for all services
type Service interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// User represents a user in the system
type User struct {
	ID        string
	Username  string
	Email     string
	Role      string
	Password  string // Hashed password
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserService defines the interface for user management
type UserService interface {
	Service
	
	// User management
	CreateUser(ctx context.Context, user *User) (*User, error)
	GetUser(ctx context.Context, userID string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	UpdateUser(ctx context.Context, user *User) (*User, error)
	DeleteUser(ctx context.Context, userID string) error
	
	// Authentication
	Authenticate(ctx context.Context, username, password string) (*User, error)
	
	// Balance management
	GetBalance(ctx context.Context, userID, currency string) (*Balance, error)
	UpdateBalance(ctx context.Context, userID, currency string, amount float64) (*Balance, error)
}

// Balance represents a user's balance in a specific currency
type Balance struct {
	UserID    string
	Currency  string
	Amount    float64
	UpdatedAt time.Time
}

// Game represents a game in the system
type Game struct {
	ID          string
	Name        string
	Type        string
	Provider    string
	Status      string
	MinBet      float64
	MaxBet      float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GameService defines the interface for game management
type GameService interface {
	Service
	
	// Game management
	CreateGame(ctx context.Context, game *Game) (*Game, error)
	GetGame(ctx context.Context, gameID string) (*Game, error)
	ListGames(ctx context.Context) ([]*Game, error)
	UpdateGame(ctx context.Context, game *Game) (*Game, error)
	DeleteGame(ctx context.Context, gameID string) error
	
	// Session management
	CreateSession(ctx context.Context, userID, gameID string) (*GameSession, error)
	GetSession(ctx context.Context, sessionID string) (*GameSession, error)
	EndSession(ctx context.Context, sessionID string) error
}

// GameSession represents an active game session
type GameSession struct {
	ID        string
	UserID    string
	GameID    string
	Status    string
	StartedAt time.Time
	EndedAt   *time.Time
}

// Transaction represents a financial transaction
type Transaction struct {
	ID            string
	UserID        string
	GameID        string
	SessionID     string
	Type          string // "bet", "win", "refund"
	Amount        float64
	Currency      string
	BalanceBefore float64
	BalanceAfter  float64
	Status        string
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// TransactionService defines the interface for transaction management
type TransactionService interface {
	Service
	
	// Transaction management
	CreateTransaction(ctx context.Context, tx *Transaction) (*Transaction, error)
	GetTransaction(ctx context.Context, txID string) (*Transaction, error)
	ListTransactions(ctx context.Context, userID string, limit int) ([]*Transaction, error)
	
	// Game transactions
	PlaceBet(ctx context.Context, userID, gameID, sessionID string, amount float64, currency string) (*Transaction, error)
	ProcessWin(ctx context.Context, userID, gameID, sessionID string, amount float64, currency string) (*Transaction, error)
	ProcessRefund(ctx context.Context, txID string) (*Transaction, error)
}