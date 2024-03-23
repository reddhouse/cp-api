package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/oklog/ulid"
	bolt "go.etcd.io/bbolt"
)

type exim struct {
	Author     string `json:"author"`
	IsApproved bool   `json:"isApproved"`
	Target     string `json:"target"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Paragraph1 string `json:"paragraph1"`
	Paragraph2 string `json:"paragraph2"`
	Paragraph3 string `json:"paragraph3"`
	Link       string `json:"link"`
}

func handleCreateExim(w http.ResponseWriter, req *http.Request) {
	type requestBody struct {
		Target     string `json:"target"`
		Title      string `json:"title"`
		Summary    string `json:"summary"`
		Paragraph1 string `json:"paragraph1"`
		Paragraph2 string `json:"paragraph2"`
		Paragraph3 string `json:"paragraph3"`
		Link       string `json:"link"`
	}
	type responseBody struct {
		EximId string `json:"eximId"`
	}
	var requestBodyInst requestBody
	var eximInst exim
	var userId ulid.ULID
	var responseBodyInst responseBody

	fmt.Printf("[api] handling POST to %s [%s]\n", req.URL.Path, cts())

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode & unmarshal JSON request body (stream) into requestBody struct.
	if err := unmarshalJson(w, &requestBodyInst, req); err != nil {
		return
	}

	// Get userId from context provided by authMiddleware and assert type.
	if err := setUserIdFromContext(w, &userId, req); err != nil {
		return
	}

	// Create ULID and db key(s).
	id, binId := createUlid()

	// Update instance fields.
	eximInst.Author = userId.String()
	eximInst.IsApproved = false
	eximInst.Target = requestBodyInst.Target
	eximInst.Title = requestBodyInst.Title
	eximInst.Summary = requestBodyInst.Summary
	eximInst.Paragraph1 = requestBodyInst.Paragraph1
	eximInst.Paragraph2 = requestBodyInst.Paragraph2
	eximInst.Paragraph3 = requestBodyInst.Paragraph3
	eximInst.Link = requestBodyInst.Link

	// Marshal mEximGrp to be stored.
	eximJs, err := json.Marshal(eximInst)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write mExim to database.
	err = db.Update(func(tx *bolt.Tx) error {
		// Retrieve buckets.
		eb := tx.Bucket([]byte("MOD_EXIM"))

		// TODO: Delete me.
		stats := eb.Stats()
		fmt.Printf("[api] There are currently %d keys in the MOD_EXIM bucket [%s]\n", stats.KeyN, cts())

		// Write key/value pair.
		if err := eb.Put(binId, eximJs); err != nil {
			return err
		}

		return nil
	})

	// Handle database error.
	if err != nil {
		fmt.Printf("[err][api] updating db with new exim: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	responseBodyInst.EximId = id.String()

	// Success. Reply with userId.
	encodeJsonAndRespond(w, responseBodyInst)
}
