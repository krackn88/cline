//! AI Processing Library implemented in Rust
//! 
//! This library provides high-performance AI text processing capabilities
//! that can be called from Go through FFI.

use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_double};
use std::slice;

#[repr(C)]
pub struct TokenizationResult {
    tokens_ptr: *mut u32,
    tokens_count: usize,
    error_message: *mut c_char,
}

// Free memory allocated for TokenizationResult
#[no_mangle]
pub extern "C" fn free_tokenization_result(result: TokenizationResult) {
    // Free the tokens array if it exists
    if !result.tokens_ptr.is_null() {
        unsafe {
            let _ = Vec::from_raw_parts(
                result.tokens_ptr,
                result.tokens_count,
                result.tokens_count,
            );
        }
    }

    // Free the error message if it exists
    if !result.error_message.is_null() {
        unsafe {
            let _ = CString::from_raw(result.error_message);
        }
    }
}

/// Tokenize a text string
///
/// Takes a text string and converts it into token IDs.
/// Returns a TokenizationResult containing the token IDs and any error message.
#[no_mangle]
pub extern "C" fn tokenize_text(text: *const c_char) -> TokenizationResult {
    // Convert C string to Rust string
    let c_str = unsafe {
        if text.is_null() {
            return TokenizationResult {
                tokens_ptr: std::ptr::null_mut(),
                tokens_count: 0,
                error_message: CString::new("Input text is null")
                    .unwrap()
                    .into_raw(),
            };
        }
        
        CStr::from_ptr(text)
    };

    let text_str = match c_str.to_str() {
        Ok(s) => s,
        Err(_) => {
            return TokenizationResult {
                tokens_ptr: std::ptr::null_mut(),
                tokens_count: 0,
                error_message: CString::new("Invalid UTF-8 in input text")
                    .unwrap()
                    .into_raw(),
            };
        }
    };

    // Simple tokenization (just for demonstration - not a real tokenizer)
    let tokens: Vec<u32> = text_str
        .split_whitespace()
        .enumerate()
        .map(|(i, _)| i as u32 + 1)
        .collect();

    // Convert the vector into a raw pointer to return
    let tokens_count = tokens.len();
    let tokens_ptr = Box::into_raw(tokens.into_boxed_slice()) as *mut u32;

    TokenizationResult {
        tokens_ptr,
        tokens_count,
        error_message: std::ptr::null_mut(),
    }
}

/// Calculate the probability distribution over the next token
///
/// Takes the token IDs processed so far and calculates the probabilities for the next token.
/// Returns an array of probabilities and its length.
#[no_mangle]
pub extern "C" fn calculate_next_token_probs(
    tokens: *const u32,
    token_count: usize,
    temperature: c_double,
    probabilities_out: *mut *mut c_double,
    prob_count_out: *mut usize,
) -> *mut c_char {
    // Safety checks
    if tokens.is_null() || probabilities_out.is_null() || prob_count_out.is_null() {
        return CString::new("Null pointer provided to calculate_next_token_probs")
            .unwrap()
            .into_raw();
    }

    // Access the tokens slice
    let token_slice = unsafe { slice::from_raw_parts(tokens, token_count) };

    // In a real implementation, we would use a language model to calculate probabilities
    // For demonstration, we'll generate some fake probabilities based on the input
    let vocab_size = 100;
    let mut probs = vec![0.01f64; vocab_size];
    
    // Simple logic to make the probability distribution depend on the input
    for &token in token_slice {
        let idx = token as usize % vocab_size;
        probs[idx] += 0.1 * temperature;
    }
    
    // Normalize the probabilities
    let sum: f64 = probs.iter().sum();
    for p in &mut probs {
        *p /= sum;
    }

    // Convert to raw pointer for returning
    let probs_ptr = Box::into_raw(probs.into_boxed_slice()) as *mut c_double;
    
    // Set output parameters
    unsafe {
        *probabilities_out = probs_ptr;
        *prob_count_out = vocab_size;
    }
    
    // No error
    std::ptr::null_mut()
}

/// Free a C string that was allocated by Rust
#[no_mangle]
pub extern "C" fn free_string(s: *mut c_char) {
    if !s.is_null() {
        unsafe {
            let _ = CString::from_raw(s);
        }
    }
}

/// Free a double array that was allocated by Rust
#[no_mangle]
pub extern "C" fn free_double_array(array: *mut c_double, _length: usize) {
    if !array.is_null() {
        unsafe {
            let _ = Vec::from_raw_parts(array, _length, _length);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::ffi::CString;
    
    #[test]
    fn test_tokenize_text() {
        let text = CString::new("Hello world").unwrap();
        let result = tokenize_text(text.as_ptr());
        
        assert!(result.error_message.is_null(), "Unexpected error");
        assert_eq!(result.tokens_count, 2, "Expected 2 tokens");
        
        unsafe {
            let tokens = slice::from_raw_parts(result.tokens_ptr, result.tokens_count);
            assert_eq!(tokens, &[1, 2], "Unexpected token IDs");
        }
        
        free_tokenization_result(result);
    }

    #[test]
    fn test_calculate_next_token_probs() {
        let tokens = vec![1u32, 2, 3];
        let mut probs_ptr: *mut c_double = std::ptr::null_mut();
        let mut prob_count: usize = 0;
        
        let error = calculate_next_token_probs(
            tokens.as_ptr(),
            tokens.len(),
            1.0,
            &mut probs_ptr,
            &mut prob_count,
        );
        
        assert!(error.is_null(), "Unexpected error");
        assert_eq!(prob_count, 100, "Expected 100 probabilities");
        assert!(!probs_ptr.is_null(), "Probabilities pointer is null");
        
        // Free the allocated memory
        free_double_array(probs_ptr, prob_count);
    }
}
