package main

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"

	"github.com/oklog/ulid"
)

type errorResponse struct {
	Error string `json:"error"`
}

func sendErrorResponse(w http.ResponseWriter, err error, statusCode int) {
	errRes := errorResponse{Error: err.Error()}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errRes)
}

func verifyContentType(w http.ResponseWriter, req *http.Request) error {
	contentType := req.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	// Unrecognized media type.
	if err != nil {
		fmt.Printf("[err][api] parsing media type: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusBadRequest)
		return err
	}
	// Unsupported media type.
	if mediaType != "application/json" {
		err := fmt.Errorf("expected application/json Content-Type")
		fmt.Printf("[err][api] verifying content type: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusUnsupportedMediaType)
		return err
	}
	return nil
}

// Decodes and unmarshals the JSON request body into the provided destination.
func unmarshalJson(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal JSON into struct: %w", err)
		fmt.Printf("[err][api] decoding JSON: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusBadRequest)
		return err
	}
	return nil
}

// Unmarshals a ULID from a string into a ulid.ULID type (16-byte array).
func unmarshalUlid(w http.ResponseWriter, dst *ulid.ULID, strId string) error {
	id, err := ulid.ParseStrict(strId)
	if err != nil {
		err = fmt.Errorf("failed to parse ULID from string: %v", err)
		fmt.Printf("[err][api] unmarshaling ULID: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return err
	}
	*dst = id
	return nil
}

// Encodes and responds with the provided struct as JSON.
func encodeJsonAndRespond(w http.ResponseWriter, src interface{}) error {
	enc := json.NewEncoder(w)
	w.Header().Set("Content-Type", "application/json")
	// Write JSON directly to the ResponseWriter.
	err := enc.Encode(src)
	if err != nil {
		err = fmt.Errorf("failed to marshal struct into JSON: %w", err)
		fmt.Printf("[err][api] encoding JSON: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return err
	}
	return nil
}

// Convert ulid to byte slice to use as db key.
func getBinId(w http.ResponseWriter, ulid ulid.ULID) ([]byte, error) {
	binId, err := ulid.MarshalBinary()
	if err != nil {
		err = fmt.Errorf("failed to marshal ULID to binary: %w", err)
		fmt.Printf("[err][api] marshaling ULID: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return nil, err
	}
	return binId, nil
}
