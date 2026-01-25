package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/burka/execbox-cloud/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountService_ListAPIKeys_Success(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account
	accountID := uuid.New()
	primaryKey := &db.APIKey{
		ID:        accountID,
		Key:       "sk_test_primary",
		Tier:      "free",
		IsActive:  true,
		AccountID: accountID, // Primary keys have account_id == id
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey.Key] = primaryKey

	// Create additional keys for the same account
	key1Name := "Key 1"
	key1Desc := "First key"
	key1 := &db.APIKey{
		ID:          uuid.New(),
		Key:         "sk_test_key1",
		Tier:        "free",
		IsActive:    true,
		AccountID:   accountID,
		ParentKeyID: &accountID,
		Name:        &key1Name,
		Description: &key1Desc,
		CreatedAt:   time.Now().UTC(),
	}
	mockDB.apiKeysByString[key1.Key] = key1

	key2Name := "Key 2"
	key2 := &db.APIKey{
		ID:          uuid.New(),
		Key:         "sk_test_key2",
		Tier:        "free",
		IsActive:    false,
		AccountID:   accountID,
		ParentKeyID: &accountID,
		Name:        &key2Name,
		CreatedAt:   time.Now().UTC(),
	}
	mockDB.apiKeysByString[key2.Key] = key2

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), accountID)

	input := &ListAPIKeysInput{}
	output, err := service.ListAPIKeys(ctx, input)

	require.NoError(t, err)
	assert.Len(t, output.Body.Keys, 3)

	// Verify all keys returned
	keyIDs := make(map[string]bool)
	for _, k := range output.Body.Keys {
		keyIDs[k.ID] = true
	}
	assert.True(t, keyIDs[accountID.String()])
	assert.True(t, keyIDs[key1.ID.String()])
	assert.True(t, keyIDs[key2.ID.String()])
}

func TestAccountService_ListAPIKeys_Unauthorized(t *testing.T) {
	mockDB := newMockHandlerDB()
	service := NewAccountService(mockDB)

	// Call without API key context
	ctx := context.Background()
	input := &ListAPIKeysInput{}

	_, err := service.ListAPIKeys(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestAccountService_CreateAPIKey_Success(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account
	accountID := uuid.New()
	primaryKey := &db.APIKey{
		ID:        accountID,
		Key:       "sk_test_primary",
		Tier:      "free",
		IsActive:  true,
		AccountID: accountID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey.Key] = primaryKey

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), accountID)

	description := "Test key description"
	input := &CreateAPIKeyInput{
		Body: CreateAPIKeyRequest{
			Name:        "Test Key",
			Description: &description,
		},
	}

	output, err := service.CreateAPIKey(ctx, input)

	require.NoError(t, err)
	assert.NotEmpty(t, output.Body.ID)
	assert.NotEmpty(t, output.Body.Key)
	assert.Contains(t, output.Body.Key, "sk_test_")
	assert.NotNil(t, output.Body.Name)
	assert.Equal(t, "Test Key", *output.Body.Name)
	assert.NotNil(t, output.Body.Description)
	assert.Equal(t, "Test key description", *output.Body.Description)
	assert.True(t, output.Body.IsActive)
}

func TestAccountService_CreateAPIKey_MissingName(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account
	accountID := uuid.New()
	primaryKey := &db.APIKey{
		ID:        accountID,
		Key:       "sk_test_primary",
		Tier:      "free",
		IsActive:  true,
		AccountID: accountID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey.Key] = primaryKey

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), accountID)

	input := &CreateAPIKeyInput{
		Body: CreateAPIKeyRequest{
			Name: "", // Empty name
		},
	}

	_, err := service.CreateAPIKey(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestAccountService_GetAPIKey_Success(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account
	accountID := uuid.New()
	primaryKey := &db.APIKey{
		ID:        accountID,
		Key:       "sk_test_primary",
		Tier:      "free",
		IsActive:  true,
		AccountID: accountID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey.Key] = primaryKey

	// Create a specific key to retrieve
	targetKeyID := uuid.New()
	targetKeyName := "Target Key"
	targetKeyDesc := "Target description"
	targetKey := &db.APIKey{
		ID:          targetKeyID,
		Key:         "sk_test_target",
		Tier:        "free",
		IsActive:    true,
		AccountID:   accountID,
		ParentKeyID: &accountID,
		Name:        &targetKeyName,
		Description: &targetKeyDesc,
		CreatedAt:   time.Now().UTC(),
	}
	mockDB.apiKeysByString[targetKey.Key] = targetKey

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), accountID)

	input := &GetAPIKeyInput{
		ID: targetKeyID.String(),
	}

	output, err := service.GetAPIKey(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, targetKeyID.String(), output.Body.ID)
	assert.NotNil(t, output.Body.Name)
	assert.Equal(t, "Target Key", *output.Body.Name)
	assert.NotNil(t, output.Body.Description)
	assert.Equal(t, "Target description", *output.Body.Description)
	assert.Equal(t, "sk_test...rget", output.Body.KeyPreview)
}

func TestAccountService_GetAPIKey_NotFound(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account
	accountID := uuid.New()
	primaryKey := &db.APIKey{
		ID:        accountID,
		Key:       "sk_test_primary",
		Tier:      "free",
		IsActive:  true,
		AccountID: accountID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey.Key] = primaryKey

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), accountID)

	// Try to get a non-existent key
	nonExistentKeyID := uuid.New()
	input := &GetAPIKeyInput{
		ID: nonExistentKeyID.String(),
	}

	_, err := service.GetAPIKey(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestAccountService_GetAPIKey_WrongAccount(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account 1
	account1ID := uuid.New()
	primaryKey1 := &db.APIKey{
		ID:        account1ID,
		Key:       "sk_test_account1",
		Tier:      "free",
		IsActive:  true,
		AccountID: account1ID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey1.Key] = primaryKey1

	// Create primary key for account 2
	account2ID := uuid.New()
	primaryKey2 := &db.APIKey{
		ID:        account2ID,
		Key:       "sk_test_account2",
		Tier:      "free",
		IsActive:  true,
		AccountID: account2ID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey2.Key] = primaryKey2

	// Create a key for account 2
	key2Name := "Account 2 Key"
	key2 := &db.APIKey{
		ID:          uuid.New(),
		Key:         "sk_test_key2",
		Tier:        "free",
		IsActive:    true,
		AccountID:   account2ID,
		ParentKeyID: &account2ID,
		Name:        &key2Name,
		CreatedAt:   time.Now().UTC(),
	}
	mockDB.apiKeysByString[key2.Key] = key2

	service := NewAccountService(mockDB)
	// Authenticate as account 1
	ctx := WithAPIKeyID(context.Background(), account1ID)

	// Try to get key from account 2
	input := &GetAPIKeyInput{
		ID: key2.ID.String(),
	}

	_, err := service.GetAPIKey(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestAccountService_UpdateAPIKey_Success(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account
	accountID := uuid.New()
	primaryKey := &db.APIKey{
		ID:        accountID,
		Key:       "sk_test_primary",
		Tier:      "free",
		IsActive:  true,
		AccountID: accountID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey.Key] = primaryKey

	// Create a key to update
	targetKeyID := uuid.New()
	targetKeyName := "Old Name"
	targetKeyDesc := "Old description"
	targetKey := &db.APIKey{
		ID:          targetKeyID,
		Key:         "sk_test_target",
		Tier:        "free",
		IsActive:    true,
		AccountID:   accountID,
		ParentKeyID: &accountID,
		Name:        &targetKeyName,
		Description: &targetKeyDesc,
		CreatedAt:   time.Now().UTC(),
	}
	mockDB.apiKeysByString[targetKey.Key] = targetKey

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), accountID)

	newName := "New Name"
	newDesc := "New description"
	input := &UpdateAPIKeyInput{
		ID: targetKeyID.String(),
		Body: UpdateAPIKeyRequest{
			Name:        &newName,
			Description: &newDesc,
		},
	}

	output, err := service.UpdateAPIKey(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, targetKeyID.String(), output.Body.ID)
	assert.NotNil(t, output.Body.Name)
	assert.Equal(t, "New Name", *output.Body.Name)
	assert.NotNil(t, output.Body.Description)
	assert.Equal(t, "New description", *output.Body.Description)

	// Verify the mock was updated
	assert.NotNil(t, targetKey.Name)
	assert.Equal(t, "New Name", *targetKey.Name)
	assert.NotNil(t, targetKey.Description)
	assert.Equal(t, "New description", *targetKey.Description)
}

func TestAccountService_DeleteAPIKey_Success(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account
	accountID := uuid.New()
	primaryKey := &db.APIKey{
		ID:        accountID,
		Key:       "sk_test_primary",
		Tier:      "free",
		IsActive:  true,
		AccountID: accountID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey.Key] = primaryKey

	// Create a non-primary key to delete
	targetKeyID := uuid.New()
	targetKeyName := "Key to delete"
	targetKey := &db.APIKey{
		ID:          targetKeyID,
		Key:         "sk_test_target",
		Tier:        "free",
		IsActive:    true,
		AccountID:   accountID,
		ParentKeyID: &accountID,
		Name:        &targetKeyName,
		CreatedAt:   time.Now().UTC(),
	}
	mockDB.apiKeysByString[targetKey.Key] = targetKey

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), accountID)

	input := &DeleteAPIKeyInput{
		ID: targetKeyID.String(),
	}

	_, err := service.DeleteAPIKey(ctx, input)

	require.NoError(t, err)

	// Verify the key was deactivated
	assert.False(t, targetKey.IsActive)
	assert.NotNil(t, targetKey.LastUpdatedBy)
	assert.Equal(t, accountID.String(), *targetKey.LastUpdatedBy)
}

func TestAccountService_DeleteAPIKey_PrimaryKey(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account
	accountID := uuid.New()
	primaryKey := &db.APIKey{
		ID:        accountID,
		Key:       "sk_test_primary",
		Tier:      "free",
		IsActive:  true,
		AccountID: accountID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey.Key] = primaryKey

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), accountID)

	// Try to delete the primary key
	input := &DeleteAPIKeyInput{
		ID: accountID.String(),
	}

	_, err := service.DeleteAPIKey(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete the primary API key")

	// Verify the key was NOT deactivated
	assert.True(t, primaryKey.IsActive)
}

func TestAccountService_RotateAPIKey_Success(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account
	accountID := uuid.New()
	primaryKey := &db.APIKey{
		ID:        accountID,
		Key:       "sk_test_primary",
		Tier:      "free",
		IsActive:  true,
		AccountID: accountID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey.Key] = primaryKey

	// Create a key to rotate
	targetKeyID := uuid.New()
	targetKeyName := "Key to rotate"
	originalKey := "sk_test_original"
	targetKey := &db.APIKey{
		ID:          targetKeyID,
		Key:         originalKey,
		Tier:        "free",
		IsActive:    true,
		AccountID:   accountID,
		ParentKeyID: &accountID,
		Name:        &targetKeyName,
		CreatedAt:   time.Now().UTC(),
	}
	mockDB.apiKeysByString[targetKey.Key] = targetKey

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), accountID)

	input := &RotateAPIKeyInput{
		ID: targetKeyID.String(),
	}

	output, err := service.RotateAPIKey(ctx, input)

	require.NoError(t, err)
	assert.Equal(t, targetKeyID.String(), output.Body.ID)
	assert.NotEmpty(t, output.Body.Key)
	assert.NotEqual(t, originalKey, output.Body.Key) // Key should be different
	assert.Contains(t, output.Body.Key, "sk_test_")  // Should have correct prefix
	assert.NotNil(t, output.Body.Name)
	assert.Equal(t, "Key to rotate", *output.Body.Name) // Name should be preserved
	assert.True(t, output.Body.IsActive)                // Should remain active

	// Verify the old key was removed from the map
	_, oldKeyExists := mockDB.apiKeysByString[originalKey]
	assert.False(t, oldKeyExists, "Old key should be removed from map")

	// Verify the new key exists in the map
	_, newKeyExists := mockDB.apiKeysByString[output.Body.Key]
	assert.True(t, newKeyExists, "New key should exist in map")
}

func TestCreateAPIKey_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    CreateAPIKeyRequest
		wantErr string
	}{
		{
			name:    "empty name",
			body:    CreateAPIKeyRequest{Name: ""},
			wantErr: "name is required",
		},
		{
			name:    "name too long",
			body:    CreateAPIKeyRequest{Name: strings.Repeat("a", 101)},
			wantErr: "name must be 100 characters or less",
		},
		{
			name: "description too long",
			body: CreateAPIKeyRequest{
				Name:        "Test Key",
				Description: stringPtr(strings.Repeat("a", 501)),
			},
			wantErr: "description must be 500 characters or less",
		},
		{
			name: "custom daily limit zero",
			body: CreateAPIKeyRequest{
				Name:             "Test Key",
				CustomDailyLimit: intPtr(0),
			},
			wantErr: "custom_daily_limit must be greater than 0",
		},
		{
			name: "custom daily limit negative",
			body: CreateAPIKeyRequest{
				Name:             "Test Key",
				CustomDailyLimit: intPtr(-1),
			},
			wantErr: "custom_daily_limit must be greater than 0",
		},
		{
			name: "custom concurrent limit zero",
			body: CreateAPIKeyRequest{
				Name:                  "Test Key",
				CustomConcurrentLimit: intPtr(0),
			},
			wantErr: "custom_concurrent_limit must be greater than 0",
		},
		{
			name: "custom concurrent limit negative",
			body: CreateAPIKeyRequest{
				Name:                  "Test Key",
				CustomConcurrentLimit: intPtr(-1),
			},
			wantErr: "custom_concurrent_limit must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := newMockHandlerDB()

			// Create primary key for account
			accountID := uuid.New()
			primaryKey := &db.APIKey{
				ID:        accountID,
				Key:       "sk_test_primary",
				Tier:      "free",
				IsActive:  true,
				AccountID: accountID,
				CreatedAt: time.Now().UTC(),
			}
			mockDB.apiKeysByString[primaryKey.Key] = primaryKey

			service := NewAccountService(mockDB)
			ctx := WithAPIKeyID(context.Background(), accountID)

			input := &CreateAPIKeyInput{Body: tt.body}
			_, err := service.CreateAPIKey(ctx, input)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestUpdateAPIKey_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    UpdateAPIKeyRequest
		wantErr string
	}{
		{
			name:    "empty name update",
			body:    UpdateAPIKeyRequest{Name: stringPtr("")},
			wantErr: "", // Update with empty name is allowed (allows clearing the name)
		},
		{
			name:    "name too long",
			body:    UpdateAPIKeyRequest{Name: stringPtr(strings.Repeat("a", 101))},
			wantErr: "name must be 100 characters or less",
		},
		{
			name: "description too long",
			body: UpdateAPIKeyRequest{
				Name:        stringPtr("Valid Name"),
				Description: stringPtr(strings.Repeat("a", 501)),
			},
			wantErr: "description must be 500 characters or less",
		},
		{
			name: "custom daily limit zero",
			body: UpdateAPIKeyRequest{
				Name:             stringPtr("Valid Name"),
				CustomDailyLimit: intPtr(0),
			},
			wantErr: "custom_daily_limit must be greater than 0",
		},
		{
			name: "custom concurrent limit negative",
			body: UpdateAPIKeyRequest{
				Name:                  stringPtr("Valid Name"),
				CustomConcurrentLimit: intPtr(-1),
			},
			wantErr: "custom_concurrent_limit must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := newMockHandlerDB()

			// Create primary key for account
			accountID := uuid.New()
			primaryKey := &db.APIKey{
				ID:        accountID,
				Key:       "sk_test_primary",
				Tier:      "free",
				IsActive:  true,
				AccountID: accountID,
				CreatedAt: time.Now().UTC(),
			}
			mockDB.apiKeysByString[primaryKey.Key] = primaryKey

			// Create a key to update
			targetKeyID := uuid.New()
			targetKeyName := "Original Name"
			targetKey := &db.APIKey{
				ID:          targetKeyID,
				Key:         "sk_test_target",
				Tier:        "free",
				IsActive:    true,
				AccountID:   accountID,
				ParentKeyID: &accountID,
				Name:        &targetKeyName,
				CreatedAt:   time.Now().UTC(),
			}
			mockDB.apiKeysByString[targetKey.Key] = targetKey

			service := NewAccountService(mockDB)
			ctx := WithAPIKeyID(context.Background(), accountID)

			input := &UpdateAPIKeyInput{
				ID:   targetKeyID.String(),
				Body: tt.body,
			}
			_, err := service.UpdateAPIKey(ctx, input)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestDeleteAPIKey_AlreadyDeleted(t *testing.T) {
	mockDB := newMockHandlerDB()

	// Create primary key for account
	accountID := uuid.New()
	primaryKey := &db.APIKey{
		ID:        accountID,
		Key:       "sk_test_primary",
		Tier:      "free",
		IsActive:  true,
		AccountID: accountID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey.Key] = primaryKey

	// Create a key that is already deactivated
	targetKeyID := uuid.New()
	targetKeyName := "Deleted Key"
	targetKey := &db.APIKey{
		ID:          targetKeyID,
		Key:         "sk_test_target",
		Tier:        "free",
		IsActive:    false, // Already deactivated
		AccountID:   accountID,
		ParentKeyID: &accountID,
		Name:        &targetKeyName,
		CreatedAt:   time.Now().UTC(),
	}
	mockDB.apiKeysByString[targetKey.Key] = targetKey

	service := NewAccountService(mockDB)
	ctx := WithAPIKeyID(context.Background(), accountID)

	input := &DeleteAPIKeyInput{
		ID: targetKeyID.String(),
	}

	// Delete the already-deleted key - should succeed
	_, err := service.DeleteAPIKey(ctx, input)

	require.NoError(t, err)
	// Verify it remains inactive
	assert.False(t, targetKey.IsActive)
}

func TestGetAPIKey_WrongAccount_Variation(t *testing.T) {
	// This test is already present, but adding a variation with more detailed verification
	mockDB := newMockHandlerDB()

	// Create primary key for account 1
	account1ID := uuid.New()
	primaryKey1 := &db.APIKey{
		ID:        account1ID,
		Key:       "sk_test_account1",
		Tier:      "free",
		IsActive:  true,
		AccountID: account1ID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey1.Key] = primaryKey1

	// Create primary key for account 2
	account2ID := uuid.New()
	primaryKey2 := &db.APIKey{
		ID:        account2ID,
		Key:       "sk_test_account2",
		Tier:      "free",
		IsActive:  true,
		AccountID: account2ID,
		CreatedAt: time.Now().UTC(),
	}
	mockDB.apiKeysByString[primaryKey2.Key] = primaryKey2

	// Create a secondary key for account 2
	key2ID := uuid.New()
	key2Name := "Account 2 Secondary Key"
	key2 := &db.APIKey{
		ID:          key2ID,
		Key:         "sk_test_key2",
		Tier:        "free",
		IsActive:    true,
		AccountID:   account2ID,
		ParentKeyID: &account2ID,
		Name:        &key2Name,
		CreatedAt:   time.Now().UTC(),
	}
	mockDB.apiKeysByString[key2.Key] = key2

	service := NewAccountService(mockDB)
	// Authenticate as account 1
	ctx := WithAPIKeyID(context.Background(), account1ID)

	// Try to get secondary key from account 2
	input := &GetAPIKeyInput{
		ID: key2ID.String(),
	}

	_, err := service.GetAPIKey(ctx, input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// Helper functions for pointer creation
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
