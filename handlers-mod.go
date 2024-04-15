package main

import (
	"fmt"
	"net/http"

	"github.com/oklog/ulid"
)

func handleCreateExim(w http.ResponseWriter, req *http.Request) {
	type ReqBody struct {
		Target     string `json:"target"`
		Title      string `json:"title"`
		Summary    string `json:"summary"`
		Paragraph1 string `json:"paragraph1"`
		Paragraph2 string `json:"paragraph2"`
		Paragraph3 string `json:"paragraph3"`
		Link       string `json:"link"`
	}
	type ResBody struct {
		EximId string `json:"eximId"`
	}
	var reqBody ReqBody
	var exim *Exim = new(Exim)
	var userId ulid.ULID
	var resBody ResBody

	fmt.Printf("[api] handling POST to %s [%s]\n", req.URL.Path, cts())

	// Enforce JSON Content-Type.
	if err := verifyContentType(w, req); err != nil {
		return
	}
	// Decode & unmarshal JSON request body (stream) into ReqBody struct.
	if err := unmarshalJson(w, &reqBody, req); err != nil {
		return
	}

	// Get/set userId from context provided by authMiddleware and assert type.
	if err := setUserIdFromContext(w, &userId, req); err != nil {
		return
	}

	// Create ULID and db key(s).
	id, binId := createUlid()

	// Update instance fields.
	exim.EximId = id
	exim.Author = userId.String()
	exim.IsApproved = false
	exim.Target = reqBody.Target
	exim.Title = reqBody.Title
	exim.Summary = reqBody.Summary
	exim.Paragraph1 = reqBody.Paragraph1
	exim.Paragraph2 = reqBody.Paragraph2
	exim.Paragraph3 = reqBody.Paragraph3
	exim.Link = reqBody.Link

	// Execute db transaction.
	err := exim.createEximTx(binId)
	if err != nil {
		fmt.Printf("[err][api] updating db with new exim: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	resBody.EximId = exim.EximId.String()

	// Success. Reply with userId.
	encodeJsonAndRespond(w, resBody)
}

func handleGetExims(w http.ResponseWriter, req *http.Request) {
	type ResBody struct {
		Exims Exims `json:"exims"`
	}
	var resBody ResBody

	fmt.Printf("[api] handling GET to %s [%s]\n", req.URL.Path, cts())

	// Execute db transaction.
	err := resBody.Exims.getEximsTx()
	if err != nil {
		fmt.Printf("[err][api] fetching exims: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	// Success. Reply with exims.
	encodeJsonAndRespond(w, resBody)
}

func handleGetEximDetails(w http.ResponseWriter, req *http.Request) {
	fmt.Printf("[api] handling GET to %s [%s]\n", req.URL.Path, cts())
	var exim *Exim = new(Exim)
	eximId := req.PathValue("ulid")

	// Decode & unmarshal ulid from string into exim.EximId.
	if err := unmarshalUlid(w, &exim.EximId, eximId); err != nil {
		return
	}

	// Convert ulid to byte slice to use as db key.
	eximBinId, err := getBinId(w, exim.EximId)
	if err != nil {
		return
	}

	// Execute db transaction.
	err = exim.getEximDetailsTx(eximBinId)
	if err != nil {
		fmt.Printf("[err][api] fetching exim details: %v [%s]\n", err, cts())
		sendErrorResponse(w, err, http.StatusInternalServerError)
		return
	}

	// Success. Reply with exim details.
	encodeJsonAndRespond(w, exim)
}
