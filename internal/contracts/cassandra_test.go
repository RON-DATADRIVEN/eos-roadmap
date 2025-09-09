package contracts

import (
	"testing"
	"time"
)

// Test fixtures for payload testing
var (
	validPayloadFixture = &Payload{
		ID:        "test-payload-123",
		Data:      "sample test data",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Type:      "test",
		Version:   1,
	}
	
	updatePayloadFixture = &Payload{
		ID:        "test-payload-123",
		Data:      "updated test data",
		Timestamp: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
		Type:      "test-updated",
		Version:   2,
	}
	
	nilPayloadFixture *Payload = nil
	
	emptyIDPayloadFixture = &Payload{
		ID:        "",
		Data:      "test data",
		Timestamp: time.Now(),
		Type:      "test",
		Version:   1,
	}
	
	emptyDataPayloadFixture = &Payload{
		ID:        "test-id",
		Data:      "",
		Timestamp: time.Now(),
		Type:      "test",
		Version:   1,
	}
)

// TestPayloadDAO_Insert tests the contract for Insert operations
func TestPayloadDAO_Insert(t *testing.T) {
	dao := NewPayloadDAO()
	
	tests := []struct {
		name         string
		payload      *Payload
		expectedErr  error
		description  string
	}{
		{
			name:        "successful_insert",
			payload:     validPayloadFixture,
			expectedErr: nil,
			description: "Contract: valid payload should be inserted successfully",
		},
		{
			name:        "nil_payload_error",
			payload:     nilPayloadFixture,
			expectedErr: ErrNilPayload,
			description: "Contract: nil payload should return ErrNilPayload",
		},
		{
			name:        "empty_id_error",
			payload:     emptyIDPayloadFixture,
			expectedErr: ErrEmptyID,
			description: "Contract: payload with empty ID should return ErrEmptyID",
		},
		{
			name:        "empty_data_error",
			payload:     emptyDataPayloadFixture,
			expectedErr: ErrEmptyData,
			description: "Contract: payload with empty data should return ErrEmptyData",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dao.Insert(tt.payload)
			
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

// TestPayloadDAO_Select tests the contract for Select operations
func TestPayloadDAO_Select(t *testing.T) {
	dao := NewPayloadDAO()
	
	tests := []struct {
		name        string
		id          string
		expectedErr error
		description string
	}{
		{
			name:        "successful_select",
			id:          "test-payload-123",
			expectedErr: nil,
			description: "Contract: valid ID should return payload successfully",
		},
		{
			name:        "empty_id_error",
			id:          "",
			expectedErr: ErrEmptyID,
			description: "Contract: empty ID should return ErrEmptyID",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := dao.Select(tt.id)
			
			if tt.expectedErr != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil. %s", tt.expectedErr, tt.description)
					return
				}
				if err != tt.expectedErr {
					t.Errorf("Expected error %v, got %v. %s", tt.expectedErr, err, tt.description)
					return
				}
				if payload != nil {
					t.Errorf("Expected nil payload on error, got %v. %s", payload, tt.description)
					return
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v. %s", err, tt.description)
					return
				}
				if payload == nil {
					t.Errorf("Expected payload, got nil. %s", tt.description)
					return
				}
				if payload.ID != tt.id {
					t.Errorf("Expected payload ID %s, got %s. %s", tt.id, payload.ID, tt.description)
					return
				}
			}
		})
	}
}

// TestPayloadDAO_Update tests the contract for Update operations
func TestPayloadDAO_Update(t *testing.T) {
	dao := NewPayloadDAO()
	
	tests := []struct {
		name         string
		payload      *Payload
		expectedErr  error
		description  string
	}{
		{
			name:        "successful_update",
			payload:     updatePayloadFixture,
			expectedErr: nil,
			description: "Contract: valid payload should be updated successfully",
		},
		{
			name:        "nil_payload_error",
			payload:     nilPayloadFixture,
			expectedErr: ErrNilPayload,
			description: "Contract: nil payload should return ErrNilPayload",
		},
		{
			name:        "empty_id_error",
			payload:     emptyIDPayloadFixture,
			expectedErr: ErrEmptyID,
			description: "Contract: payload with empty ID should return ErrEmptyID",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dao.Update(tt.payload)
			
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

// TestPayloadDAO_Delete tests the contract for Delete operations
func TestPayloadDAO_Delete(t *testing.T) {
	dao := NewPayloadDAO()
	
	tests := []struct {
		name        string
		id          string
		expectedErr error
		description string
	}{
		{
			name:        "successful_delete",
			id:          "test-payload-123",
			expectedErr: nil,
			description: "Contract: valid ID should be deleted successfully",
		},
		{
			name:        "empty_id_error",
			id:          "",
			expectedErr: ErrEmptyID,
			description: "Contract: empty ID should return ErrEmptyID",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := dao.Delete(tt.id)
			
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

// TestPayloadDAO_SelectAll tests the contract for SelectAll operations
func TestPayloadDAO_SelectAll(t *testing.T) {
	dao := NewPayloadDAO()
	
	t.Run("successful_select_all", func(t *testing.T) {
		payloads, err := dao.SelectAll()
		
		if err != nil {
			t.Errorf("Expected no error, got %v. Contract: SelectAll should return all payloads successfully", err)
			return
		}
		
		if payloads == nil {
			t.Errorf("Expected payloads slice, got nil. Contract: SelectAll should return non-nil slice")
			return
		}
		
		if len(payloads) == 0 {
			t.Errorf("Expected payloads, got empty slice. Contract: SelectAll should return mock payloads for testing")
			return
		}
		
		// Verify payload structure
		for i, payload := range payloads {
			if payload == nil {
				t.Errorf("Payload at index %d is nil. Contract: all payloads should be valid", i)
				continue
			}
			if payload.ID == "" {
				t.Errorf("Payload at index %d has empty ID. Contract: all payloads should have valid IDs", i)
			}
			if payload.Data == "" {
				t.Errorf("Payload at index %d has empty data. Contract: all payloads should have valid data", i)
			}
		}
	})
}

// TestPayloadDAO_CRUDIntegration tests the contract for complete CRUD flow
func TestPayloadDAO_CRUDIntegration(t *testing.T) {
	dao := NewPayloadDAO()
	
	t.Run("crud_workflow", func(t *testing.T) {
		// Test Insert
		err := dao.Insert(validPayloadFixture)
		if err != nil {
			t.Errorf("Insert failed: %v. Contract: valid payload should be inserted", err)
			return
		}
		
		// Test Select
		payload, err := dao.Select(validPayloadFixture.ID)
		if err != nil {
			t.Errorf("Select failed: %v. Contract: inserted payload should be retrievable", err)
			return
		}
		if payload.ID != validPayloadFixture.ID {
			t.Errorf("Selected payload ID mismatch. Expected %s, got %s", validPayloadFixture.ID, payload.ID)
		}
		
		// Test Update
		err = dao.Update(updatePayloadFixture)
		if err != nil {
			t.Errorf("Update failed: %v. Contract: existing payload should be updatable", err)
			return
		}
		
		// Test Delete
		err = dao.Delete(validPayloadFixture.ID)
		if err != nil {
			t.Errorf("Delete failed: %v. Contract: existing payload should be deletable", err)
			return
		}
		
		// Test SelectAll
		payloads, err := dao.SelectAll()
		if err != nil {
			t.Errorf("SelectAll failed: %v. Contract: all payloads should be retrievable", err)
			return
		}
		if len(payloads) == 0 {
			t.Errorf("SelectAll returned empty slice. Contract: should return available payloads")
		}
	})
}

// BenchmarkPayloadDAO_Insert benchmarks the Insert operation
func BenchmarkPayloadDAO_Insert(b *testing.B) {
	dao := NewPayloadDAO()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dao.Insert(validPayloadFixture)
	}
}

// BenchmarkPayloadDAO_Select benchmarks the Select operation
func BenchmarkPayloadDAO_Select(b *testing.B) {
	dao := NewPayloadDAO()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dao.Select("test-payload-123")
	}
}