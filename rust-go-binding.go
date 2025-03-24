package rustbinding

/*
#cgo LDFLAGS: -L${SRCDIR}/../lib -laiprocessor
#include <stdlib.h>
#include <stdint.h>

typedef struct {
    uint32_t* tokens_ptr;
    size_t tokens_count;
    char* error_message;
} TokenizationResult;

// Function declarations from Rust
TokenizationResult tokenize_text(const char* text);
void free_tokenization_result(TokenizationResult result);
char* calculate_next_token_probs(const uint32_t* tokens, size_t token_count, 
                                double temperature, double** probabilities_out, 
                                size_t* prob_count_out);
void free_string(char* s);
void free_double_array(double* array, size_t length);
*/
import "C"
import (
	"errors"
	"unsafe"
)

// TokenizationResult represents the result of tokenizing text
type TokenizationResult struct {
	Tokens []uint32
	Error  error
}

// TokenizeText tokenizes the given text using the Rust implementation
func TokenizeText(text string) TokenizationResult {
	// Convert Go string to C string
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	// Call Rust function
	result := C.tokenize_text(cText)

	// Prepare return value
	var goResult TokenizationResult

	// Check for error
	if result.error_message != nil {
		goResult.Error = errors.New(C.GoString(result.error_message))
		// Free the memory allocated by Rust
		C.free_tokenization_result(result)
		return goResult
	}

	// Convert C array to Go slice
	if result.tokens_ptr != nil && result.tokens_count > 0 {
		// Create a slice that references the C array without copying
		tokenSlice := unsafe.Slice(result.tokens_ptr, result.tokens_count)
		
		// Copy the data to a Go-managed slice
		goResult.Tokens = make([]uint32, result.tokens_count)
		for i, token := range tokenSlice {
			goResult.Tokens[i] = uint32(token)
		}
	}

	// Free the memory allocated by Rust
	C.free_tokenization_result(result)
	return goResult
}

// ProbabilityDistribution holds token probabilities
type ProbabilityDistribution struct {
	Probabilities []float64
	Error         error
}

// CalculateNextTokenProbs calculates probability distribution for the next token
func CalculateNextTokenProbs(tokens []uint32, temperature float64) ProbabilityDistribution {
	var result ProbabilityDistribution

	// Handle empty tokens gracefully
	if len(tokens) == 0 {
		result.Error = errors.New("empty token sequence")
		return result
	}

	// Convert Go slice to C array
	cTokens := (*C.uint32_t)(unsafe.Pointer(&tokens[0]))
	tokenCount := C.size_t(len(tokens))

	// Prepare output parameters
	var probabilitiesOut *C.double
	var probCountOut C.size_t

	// Call Rust function
	errorMsg := C.calculate_next_token_probs(
		cTokens,
		tokenCount,
		C.double(temperature),
		&probabilitiesOut,
		&probCountOut,
	)

	// Check for error
	if errorMsg != nil {
		result.Error = errors.New(C.GoString(errorMsg))
		C.free_string(errorMsg)
		return result
	}

	// Convert C array to Go slice
	if probabilitiesOut != nil && probCountOut > 0 {
		// Create a slice that references the C array
		probSlice := unsafe.Slice(probabilitiesOut, probCountOut)
		
		// Copy to Go-managed memory
		result.Probabilities = make([]float64, probCountOut)
		for i, prob := range probSlice {
			result.Probabilities[i] = float64(prob)
		}
		
		// Free C array
		C.free_double_array(probabilitiesOut, probCountOut)
	}

	return result
}

// IsRustLibraryAvailable checks if the Rust library is available
func IsRustLibraryAvailable() bool {
	// Try to call a simple function
	cText := C.CString("test")
	defer C.free(unsafe.Pointer(cText))
	
	result := C.tokenize_text(cText)
	
	// We need to free the result regardless of the outcome
	hasError := result.error_message != nil
	C.free_tokenization_result(result)
	
	// If we got an error about the library not being found, return false
	// But for any other normal errors, the library is available
	if hasError {
		errorMsg := C.GoString(result.error_message)
		if errorMsg == "Input text is null" {
			return false
		}
	}
	
	return true
}
