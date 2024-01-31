package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
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
		err := errors.New("error expecting application/json Content-Type")
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return err
	}
	return nil
}

func decodeJsonIntoStruct(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err != nil {
		err = fmt.Errorf("error decoding JSON into struct: %w", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
	return nil
}
