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

func decodeJsonIntoDst(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err != nil {
		err = fmt.Errorf("failed to decode JSON into struct: %w", err)
		log.Printf("[error-api] decoding JSON: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
	return nil
}

func decodeUlidIntoDst(w http.ResponseWriter, r *http.Request, dstId *ulid.ULID, strId string) error {
	id, err := ulid.ParseStrict(strId)
	if err != nil {
		err = fmt.Errorf("failed to parse ULID from string: %v", err)
		log.Printf("[error-api] unmarshaling ULID: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	*dstId = id
	return nil
}
