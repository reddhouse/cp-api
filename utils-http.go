package main

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
)

type utils_http struct{}

func (u *utils_http) verifyContentType(w http.ResponseWriter, req *http.Request) error {
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
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return err
	}
	return nil
}

func (u *utils_http) decodeJsonIntoStruct(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}
	return nil
}
