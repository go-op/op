package fuego

import (
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Constructor returns new MetadataParsers instance with empty registeredParsers slice
func TestNewMetadataParsersReturnsEmptyInstance(t *testing.T) {
	parsers := NewMetadataParsers()

	require.NotNil(t, parsers)
	require.NotNil(t, parsers.registeredParsers)
	require.Empty(t, parsers.registeredParsers)
	require.NotNil(t, parsers.registeredNames)
	require.Empty(t, parsers.registeredNames)
}

// Constructor initializes registeredNames as empty map with bool values
func TestNewMetadataParsersInitializesRegisteredNamesAsEmptyMap(t *testing.T) {
	parsers := NewMetadataParsers()

	require.NotNil(t, parsers)
	require.NotNil(t, parsers.registeredNames)
	require.Empty(t, parsers.registeredNames)
}

// Zero value initialization without using constructor
func TestMetadataParsersZeroValueInitialization(t *testing.T) {
	var parsers MetadataParsers

	require.NotNil(t, &parsers)
	require.NotNil(t, parsers.registeredParsers)
	require.Empty(t, parsers.registeredParsers)
	require.NotNil(t, parsers.registeredNames)
	require.Empty(t, parsers.registeredNames)
}

// Return all registered metadata parsers from the internal slice
func TestGetRegisteredParsersReturnsAllParsers(t *testing.T) {
	mp := &MetadataParsers{
		registeredParsers: []MetadataParserEntry{
			{Name: "parser1", Parser: func(params MetadataParserParams) { return }},
			{Name: "parser2", Parser: func(params MetadataParserParams) { return }},
		},
	}

	parsers := mp.GetRegisteredParsers()

	require.NotNil(t, parsers)
	require.Len(t, parsers, 2)

	assert.Equal(t, "parser1", parsers[0].Name)
	assert.Equal(t, "parser2", parsers[1].Name)
}

// Return empty slice when no parsers are registered
func TestGetRegisteredParsersReturnsEmptySlice(t *testing.T) {
	mp := &MetadataParsers{
		registeredParsers: []MetadataParserEntry{},
	}

	parsers := mp.GetRegisteredParsers()

	require.NotNil(t, parsers)
	require.Empty(t, parsers)
}

// Verify concurrent access is properly synchronized using mutex lock
func TestGetRegisteredParsersConcurrentAccess(t *testing.T) {
	mp := &MetadataParsers{
		registeredParsers: []MetadataParserEntry{
			{Name: "parser1", Parser: func(params MetadataParserParams) { return }},
			{Name: "parser2", Parser: func(params MetadataParserParams) { return }},
		},
	}

	var wg sync.WaitGroup
	numGoroutines := 3
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			parsers := mp.GetRegisteredParsers()

			require.NotNil(t, parsers)
			require.Len(t, parsers, 2)
		}()
	}

	wg.Wait()
}

// Check that returned slice contains expected parser entries
func TestGetRegisteredParsersContainsExpectedEntries(t *testing.T) {
	mp := &MetadataParsers{
		registeredParsers: []MetadataParserEntry{
			{Name: "expectedParser1", Parser: func(params MetadataParserParams) { return }},
			{Name: "expectedParser2", Parser: func(params MetadataParserParams) { return }},
		},
	}

	parsers := mp.GetRegisteredParsers()

	require.NotNil(t, parsers)
	require.Len(t, parsers, 2)

	require.Equal(t, "expectedParser1", parsers[0].Name)
	require.Equal(t, "expectedParser2", parsers[1].Name)
}

// Verify returned slice matches original registered parsers
func TestGetRegisteredParsersMatchesOriginal(t *testing.T) {
	mp := &MetadataParsers{
		registeredParsers: []MetadataParserEntry{
			{Name: "parser1", Parser: func(params MetadataParserParams) { return }},
			{Name: "parser2", Parser: func(params MetadataParserParams) { return }},
		},
	}

	parsers := mp.GetRegisteredParsers()

	require.NotNil(t, parsers)
	require.Len(t, parsers, 2)

	if !reflect.DeepEqual(parsers, mp.registeredParsers) {
		t.Errorf("Returned parsers do not match the original registered parsers")
	}
}

// Reset clears all registered parsers from the slice
func TestResetClearsRegisteredParsers(t *testing.T) {
	mp := &MetadataParsers{
		registeredParsers: []MetadataParserEntry{
			{Name: "parser1", Parser: nil},
			{Name: "parser2", Parser: nil},
		},
		registeredNames: map[string]bool{
			"parser1": true,
			"parser2": true,
		},
	}

	mp.Reset()

	require.NotNil(t, mp.registeredParsers)
	require.NotNil(t, mp.registeredNames)

	require.Empty(t, mp.registeredParsers)
	require.Empty(t, mp.registeredNames)
}

// Reset called on nil MetadataParsers struct
func TestResetOnNilMetadataParsers(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling Reset on nil MetadataParsers")
		}
	}()

	var mp *MetadataParsers
	mp.Reset()
}

// Reset reinitializes the registeredNames map as empty
func TestResetClearsRegisteredNames(t *testing.T) {
	mp := &MetadataParsers{
		registeredParsers: []MetadataParserEntry{
			{Name: "parser1", Parser: nil},
			{Name: "parser2", Parser: nil},
		},
		registeredNames: map[string]bool{
			"parser1": true,
			"parser2": true,
		},
	}

	mp.Reset()

	require.NotNil(t, mp)
	require.Len(t, mp.registeredNames, 0)
}

// Reset called when parsers list is already empty
func TestResetWhenParsersListIsEmpty(t *testing.T) {
	mp := &MetadataParsers{
		registeredParsers: []MetadataParserEntry{},
		registeredNames:   map[string]bool{},
	}

	mp.Reset()

	require.NotNil(t, mp)
	require.Empty(t, mp.registeredParsers)
	require.Empty(t, mp.registeredNames)
}

// State remains consistent between parsers list and names map
func TestResetMaintainsConsistencyBetweenParsersAndNames(t *testing.T) {
	mp := &MetadataParsers{
		registeredParsers: []MetadataParserEntry{
			{Name: "parser1", Parser: nil},
			{Name: "parser2", Parser: nil},
		},
		registeredNames: map[string]bool{
			"parser1": true,
			"parser2": true,
		},
	}

	mp.Reset()

	require.NotNil(t, mp)
	require.Empty(t, mp.registeredParsers)
	require.Empty(t, mp.registeredNames)
}

// Register new parser at start position successfully
func TestRegisterMetadataParserAtStart(t *testing.T) {
	parserName := "testParser"

	mp := NewMetadataParsers()
	mp.InitializeMetadataParsers([]MetadataParserEntry{})

	err := mp.RegisterMetadataParser(parserName, func(params MetadataParserParams) { return }, "prepend", "")

	require.Nil(t, err)

	registeredParsers := mp.GetRegisteredParsers()

	require.NotNil(t, registeredParsers)
	require.Len(t, registeredParsers, 1)
	assert.Equal(t, parserName, registeredParsers[0].Name)
}

// Invalid position parameter returns appropriate error
func TestRegisterMetadataParserInvalidPosition(t *testing.T) {
	parserName := "testParser"
	expectedErr := "Invalid position. Use 'prepend', 'append', 'before', or 'after'"

	mp := NewMetadataParsers()
	mp.InitializeMetadataParsers([]MetadataParserEntry{})

	err := mp.RegisterMetadataParser(parserName, func(params MetadataParserParams) {
		return
	}, "invalid", "")

	require.EqualError(t, err, expectedErr)
}

// Register parser at end position with unique name
func TestRegisterMetadataParserAtEndPosition(t *testing.T) {
	mp := NewMetadataParsers()
	mp.InitializeMetadataParsers([]MetadataParserEntry{})

	mockParser := func(params MetadataParserParams) { return }
	err := mp.RegisterMetadataParser("unique-parser", mockParser, "append", "")

	assert.NoError(t, err)
	assert.Equal(t, 1, len(mp.registeredParsers))
	assert.Equal(t, "unique-parser", mp.registeredParsers[0].Name)
}

// Register parser before existing parser with valid relativeTo
func TestRegisterMetadataParserBeforeExistingParser(t *testing.T) {
	mp := NewMetadataParsers()
	mp.InitializeMetadataParsers([]MetadataParserEntry{
		{Name: "existing-parser", Parser: func(params MetadataParserParams) { return }},
	})

	mockParser := func(params MetadataParserParams) { return }
	err := mp.RegisterMetadataParser("new-parser", mockParser, "before", "existing-parser")

	assert.NoError(t, err)
	assert.Equal(t, 2, len(mp.registeredParsers))
	assert.Equal(t, "new-parser", mp.registeredParsers[0].Name)
	assert.Equal(t, "existing-parser", mp.registeredParsers[1].Name)
}

// Register parser after existing parser with valid relativeTo
func TestRegisterMetadataParserAfterRelativePosition(t *testing.T) {
	mp := NewMetadataParsers()
	mp.InitializeMetadataParsers([]MetadataParserEntry{
		{Name: "existing-parser", Parser: func(params MetadataParserParams) { return }},
	})

	mockParser := func(params MetadataParserParams) { return }
	err := mp.RegisterMetadataParser("new-parser", mockParser, "after", "existing-parser")

	assert.NoError(t, err)
	assert.Equal(t, 2, len(mp.registeredParsers))
	assert.Equal(t, "existing-parser", mp.registeredParsers[0].Name)
	assert.Equal(t, "new-parser", mp.registeredParsers[1].Name)
}

// Register duplicate parser name returns nil without modifying list
func TestRegisterDuplicateMetadataParserName(t *testing.T) {
	mp := NewMetadataParsers()
	mp.InitializeMetadataParsers([]MetadataParserEntry{
		{Name: "existing-parser", Parser: func(params MetadataParserParams) { return }},
	})

	mockParser := func(params MetadataParserParams) { return }
	err := mp.RegisterMetadataParser("existing-parser", mockParser, "prepend", "")

	assert.NoError(t, err)
	assert.Equal(t, 1, len(mp.registeredParsers))
	assert.Equal(t, "existing-parser", mp.registeredParsers[0].Name)
}
