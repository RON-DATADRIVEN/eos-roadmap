// Package contracts provides contract definitions and implementations for Cassandra database operations.
//
// This package implements data access object (DAO) patterns with contract testing for Cassandra tables.
// It includes comprehensive validation, fixtures, and both positive and negative test cases to ensure
// proper data integrity and error handling.
//
// The package currently supports two main entities:
//   - Payloads: General data payloads with versioning and timestamps
//   - Sessions: User session management with expiration and activity tracking
//
// Example usage:
//
//	// Create a new PayloadDAO
//	payloadDAO := contracts.NewPayloadDAO()
//
//	// Create a payload
//	payload := &contracts.Payload{
//		ID:        "unique-id-123",
//		Data:      "important data",
//		Timestamp: time.Now(),
//		Type:      "data",
//		Version:   1,
//	}
//
//	// Insert the payload
//	err := payloadDAO.Insert(payload)
//	if err != nil {
//		// Handle error
//	}
//
//	// Retrieve the payload
//	retrieved, err := payloadDAO.Select("unique-id-123")
//	if err != nil {
//		// Handle error
//	}
//
// Contract Validation:
//
// All DAO operations include comprehensive input validation according to their contracts.
// This ensures data integrity and provides clear error messages for invalid operations.
//
// Testing:
//
// The package includes extensive test suites with fixtures for both positive and negative cases,
// integration tests for complete CRUD workflows, and performance benchmarks.
package contracts