package main

import (
	"net/http"
)

func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	// Create a map which holds the information that we want to send in the response.
	data := envelope{
		"status":      "available",
		"environment": app.config.env,
		"version":     version,
	}
	// Pass the map to the json.Marshal() function. This returns a []byte slice
	// containing the encoded JSON. If there was an error, we log it and send the client
	// a generic error message.
	err := app.writeJSON(w, http.StatusOK, data, nil)
	if err != nil {
		// Use the new serverErrorResponse() helper.
		app.serverErrorResponse(w, r, err)
	}
}
