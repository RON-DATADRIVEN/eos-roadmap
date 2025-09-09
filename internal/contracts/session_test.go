package contracts

import (
	"testing"
	"time"
)

// Test fixtures for session testing
var (
	validSessionFixture = &Session{
		SessionID:   "session-test-123",
		UserID:      "user-test-456",
		CreatedAt:   time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		ExpiresAt:   time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		IPAddress:   "192.168.1.100",
		UserAgent:   "TestAgent/1.0",
		IsActive:    true,
	}
	
	updateSessionFixture = &Session{
		SessionID:   "session-test-123",
		UserID:      "user-test-456",
		CreatedAt:   time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		ExpiresAt:   time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC),
		IPAddress:   "192.168.1.101",
		UserAgent:   "TestAgent/2.0",
		IsActive:    false,
	}
	
	nilSessionFixture *Session = nil
	
	emptySessionIDFixture = &Session{
		SessionID:   "",
		UserID:      "user-test",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
		IPAddress:   "192.168.1.100",
		UserAgent:   "TestAgent/1.0",
		IsActive:    true,
	}
	
	emptyUserIDFixture = &Session{
		SessionID:   "session-test",
		UserID:      "",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
		IPAddress:   "192.168.1.100",
		UserAgent:   "TestAgent/1.0",
		IsActive:    true,
	}
	
	zeroCreatedAtFixture = &Session{
		SessionID:   "session-test",
		UserID:      "user-test",
		CreatedAt:   time.Time{},
		ExpiresAt:   time.Now().Add(time.Hour),
		IPAddress:   "192.168.1.100",
		UserAgent:   "TestAgent/1.0",
		IsActive:    true,
	}
	
	invalidExpiresAtFixture = &Session{
		SessionID:   "session-test",
		UserID:      "user-test",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(-time.Hour), // Expires before created
		IPAddress:   "192.168.1.100",
		UserAgent:   "TestAgent/1.0",
		IsActive:    true,
	}
)

// TestSessionDAO_CreateSession tests the contract for CreateSession operations
func TestSessionDAO_CreateSession(t *testing.T) {
	dao := NewSessionDAO()
	
	tests := []struct {
		name         string
		session      *Session
		expectedErr  error
		description  string
	}{
		{
			name:        "successful_create",
			session:     validSessionFixture,
			expectedErr: nil,
			description: "Contract: valid session should be created successfully",
		},
		{
			name:        "nil_session_error",
			session:     nilSessionFixture,
			expectedErr: ErrNilSession,
			description: "Contract: nil session should return ErrNilSession",
		},
		{
			name:        "empty_session_id_error",
			session:     emptySessionIDFixture,
			expectedErr: ErrEmptySessionID,
			description: "Contract: session with empty SessionID should return ErrEmptySessionID",
		},
		{
			name:        "empty_user_id_error",
			session:     emptyUserIDFixture,
			expectedErr: ErrEmptyUserID,
			description: "Contract: session with empty UserID should return ErrEmptyUserID",
		},
		{
			name:        "zero_created_at_error",
			session:     zeroCreatedAtFixture,
			expectedErr: ErrInvalidCreatedAt,
			description: "Contract: session with zero CreatedAt should return ErrInvalidCreatedAt",
		},
		{
			name:        "invalid_expires_at_error",
			session:     invalidExpiresAtFixture,
			expectedErr: ErrInvalidExpiresAt,
			description: "Contract: session with ExpiresAt before CreatedAt should return ErrInvalidExpiresAt",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dao.CreateSession(tt.session)
			
			if tt.expectedErr != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil. %s", tt.expectedErr, tt.description)
					return
				}
				if err != tt.expectedErr {
					t.Errorf("Expected error %v, got %v. %s", tt.expectedErr, err, tt.description)
					return
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v. %s", err, tt.description)
					return
				}
			}
		})
	}
}

// TestSessionDAO_GetSession tests the contract for GetSession operations
func TestSessionDAO_GetSession(t *testing.T) {
	dao := NewSessionDAO()
	
	tests := []struct {
		name        string
		sessionID   string
		expectedErr error
		description string
	}{
		{
			name:        "successful_get",
			sessionID:   "session-test-123",
			expectedErr: nil,
			description: "Contract: valid SessionID should return session successfully",
		},
		{
			name:        "empty_session_id_error",
			sessionID:   "",
			expectedErr: ErrEmptySessionID,
			description: "Contract: empty SessionID should return ErrEmptySessionID",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := dao.GetSession(tt.sessionID)
			
			if tt.expectedErr != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil. %s", tt.expectedErr, tt.description)
					return
				}
				if err != tt.expectedErr {
					t.Errorf("Expected error %v, got %v. %s", tt.expectedErr, err, tt.description)
					return
				}
				if session != nil {
					t.Errorf("Expected nil session on error, got %v. %s", session, tt.description)
					return
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v. %s", err, tt.description)
					return
				}
				if session == nil {
					t.Errorf("Expected session, got nil. %s", tt.description)
					return
				}
				if session.SessionID != tt.sessionID {
					t.Errorf("Expected session ID %s, got %s. %s", tt.sessionID, session.SessionID, tt.description)
					return
				}
			}
		})
	}
}

// TestSessionDAO_UpdateSession tests the contract for UpdateSession operations
func TestSessionDAO_UpdateSession(t *testing.T) {
	dao := NewSessionDAO()
	
	tests := []struct {
		name         string
		session      *Session
		expectedErr  error
		description  string
	}{
		{
			name:        "successful_update",
			session:     updateSessionFixture,
			expectedErr: nil,
			description: "Contract: valid session should be updated successfully",
		},
		{
			name:        "nil_session_error",
			session:     nilSessionFixture,
			expectedErr: ErrNilSession,
			description: "Contract: nil session should return ErrNilSession",
		},
		{
			name:        "empty_session_id_error",
			session:     emptySessionIDFixture,
			expectedErr: ErrEmptySessionID,
			description: "Contract: session with empty SessionID should return ErrEmptySessionID",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dao.UpdateSession(tt.session)
			
			if tt.expectedErr != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil. %s", tt.expectedErr, tt.description)
					return
				}
				if err != tt.expectedErr {
					t.Errorf("Expected error %v, got %v. %s", tt.expectedErr, err, tt.description)
					return
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v. %s", err, tt.description)
					return
				}
			}
		})
	}
}

// TestSessionDAO_DeleteSession tests the contract for DeleteSession operations
func TestSessionDAO_DeleteSession(t *testing.T) {
	dao := NewSessionDAO()
	
	tests := []struct {
		name        string
		sessionID   string
		expectedErr error
		description string
	}{
		{
			name:        "successful_delete",
			sessionID:   "session-test-123",
			expectedErr: nil,
			description: "Contract: valid SessionID should be deleted successfully",
		},
		{
			name:        "empty_session_id_error",
			sessionID:   "",
			expectedErr: ErrEmptySessionID,
			description: "Contract: empty SessionID should return ErrEmptySessionID",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dao.DeleteSession(tt.sessionID)
			
			if tt.expectedErr != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil. %s", tt.expectedErr, tt.description)
					return
				}
				if err != tt.expectedErr {
					t.Errorf("Expected error %v, got %v. %s", tt.expectedErr, err, tt.description)
					return
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v. %s", err, tt.description)
					return
				}
			}
		})
	}
}

// TestSessionDAO_GetActiveSessions tests the contract for GetActiveSessions operations
func TestSessionDAO_GetActiveSessions(t *testing.T) {
	dao := NewSessionDAO()
	
	t.Run("successful_get_active_sessions", func(t *testing.T) {
		sessions, err := dao.GetActiveSessions()
		
		if err != nil {
			t.Errorf("Expected no error, got %v. Contract: GetActiveSessions should return active sessions successfully", err)
			return
		}
		
		if sessions == nil {
			t.Errorf("Expected sessions slice, got nil. Contract: GetActiveSessions should return non-nil slice")
			return
		}
		
		if len(sessions) == 0 {
			t.Errorf("Expected sessions, got empty slice. Contract: GetActiveSessions should return mock sessions for testing")
			return
		}
		
		// Verify session structure and active status
		for i, session := range sessions {
			if session == nil {
				t.Errorf("Session at index %d is nil. Contract: all sessions should be valid", i)
				continue
			}
			if session.SessionID == "" {
				t.Errorf("Session at index %d has empty SessionID. Contract: all sessions should have valid SessionIDs", i)
			}
			if session.UserID == "" {
				t.Errorf("Session at index %d has empty UserID. Contract: all sessions should have valid UserIDs", i)
			}
			if !session.IsActive {
				t.Errorf("Session at index %d is not active. Contract: GetActiveSessions should only return active sessions", i)
			}
		}
	})
}

// TestSessionDAO_CRUDIntegration tests the contract for complete session CRUD flow
func TestSessionDAO_CRUDIntegration(t *testing.T) {
	dao := NewSessionDAO()
	
	t.Run("session_crud_workflow", func(t *testing.T) {
		// Test CreateSession
		err := dao.CreateSession(validSessionFixture)
		if err != nil {
			t.Errorf("CreateSession failed: %v. Contract: valid session should be created", err)
			return
		}
		
		// Test GetSession
		session, err := dao.GetSession(validSessionFixture.SessionID)
		if err != nil {
			t.Errorf("GetSession failed: %v. Contract: created session should be retrievable", err)
			return
		}
		if session.SessionID != validSessionFixture.SessionID {
			t.Errorf("Retrieved session ID mismatch. Expected %s, got %s", validSessionFixture.SessionID, session.SessionID)
		}
		
		// Test UpdateSession
		err = dao.UpdateSession(updateSessionFixture)
		if err != nil {
			t.Errorf("UpdateSession failed: %v. Contract: existing session should be updatable", err)
			return
		}
		
		// Test DeleteSession
		err = dao.DeleteSession(validSessionFixture.SessionID)
		if err != nil {
			t.Errorf("DeleteSession failed: %v. Contract: existing session should be deletable", err)
			return
		}
		
		// Test GetActiveSessions
		sessions, err := dao.GetActiveSessions()
		if err != nil {
			t.Errorf("GetActiveSessions failed: %v. Contract: active sessions should be retrievable", err)
			return
		}
		if len(sessions) == 0 {
			t.Errorf("GetActiveSessions returned empty slice. Contract: should return available active sessions")
		}
	})
}

// BenchmarkSessionDAO_CreateSession benchmarks the CreateSession operation
func BenchmarkSessionDAO_CreateSession(b *testing.B) {
	dao := NewSessionDAO()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dao.CreateSession(validSessionFixture)
	}
}

// BenchmarkSessionDAO_GetSession benchmarks the GetSession operation
func BenchmarkSessionDAO_GetSession(b *testing.B) {
	dao := NewSessionDAO()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dao.GetSession("session-test-123")
	}
}