package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"

	"github.com/oklog/ulid"
)

func verifyContentType(w http.ResponseWriter, req *http.Request) error {
	contentType := req.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	// Unrecognized media type.
	if err != nil {
		log.Printf("[error-api] parsing media type: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
	// Unsupported media type.
	if mediaType != "application/json" {
		err := errors.New("expected application/json Content-Type")
		log.Printf("[error-api] verifying content type: %v", err)
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
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
		log.Printf("[error-api] decoding JSON: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
	return nil
}

// Unmarshals a ULID from a string into a ulid.ULID type (16-byte array).
func unmarshalUlid(w http.ResponseWriter, dst *ulid.ULID, strId string) error {
	id, err := ulid.ParseStrict(strId)
	if err != nil {
		err = fmt.Errorf("failed to parse ULID from string: %v", err)
		log.Printf("[error-api] unmarshaling ULID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		log.Printf("[error-api] encoding JSON: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	return nil
}
