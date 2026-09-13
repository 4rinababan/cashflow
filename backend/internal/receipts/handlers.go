package receipts

import (
	"io"
	"net/http"

	"cashflow-backend/internal/auth"
)

// maxUploadBytes is the multipart body cap: the image cap plus a little room
// for form overhead, so ParseMultipartForm's own limit matches the image
// limit Service enforces rather than silently allowing a larger request.
const maxUploadBytes = maxImageBytes + 64*1024

const dateLayout = "2006-01-02"

type lineItemResponse struct {
	Description string `json:"description"`
	AmountMinor int64  `json:"amountMinor"`
}

type draftResponse struct {
	Merchant      string             `json:"merchant"`
	OccurredAt    *string            `json:"occurredAt"`
	Currency      string             `json:"currency"`
	TotalMinor    int64              `json:"totalMinor"`
	CategoryGuess string             `json:"categoryGuess"`
	LineItems     []lineItemResponse `json:"lineItems"`
	Confidence    float64            `json:"confidence"`
}

func toDraftResponse(d Draft) draftResponse {
	resp := draftResponse{
		Merchant:      d.Merchant,
		Currency:      d.Currency,
		TotalMinor:    d.TotalMinor,
		CategoryGuess: d.CategoryGuess,
		Confidence:    d.Confidence,
		LineItems:     make([]lineItemResponse, 0, len(d.LineItems)),
	}
	if d.OccurredAt != nil {
		formatted := d.OccurredAt.Format(dateLayout)
		resp.OccurredAt = &formatted
	}
	for _, li := range d.LineItems {
		resp.LineItems = append(resp.LineItems, lineItemResponse{Description: li.Description, AmountMinor: li.AmountMinor})
	}
	return resp
}

// ScanHandler handles POST /receipts/scan: a multipart/form-data request
// with one "image" file field. It returns a Draft for the app to pre-fill
// into the existing new-transaction form — it never saves a transaction
// itself (docs/adr/0002-ai-receipt-scanning.md).
func ScanHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserIDFromContext(r.Context())
		if !ok {
			respondJSON(w, http.StatusUnauthorized, errorBody("not authenticated"))
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			respondJSON(w, http.StatusBadRequest, errorBody("image is too large, or the request is not multipart/form-data"))
			return
		}

		file, header, err := r.FormFile("image")
		if err != nil {
			respondJSON(w, http.StatusBadRequest, errorBody(`an "image" file field is required`))
			return
		}
		defer file.Close()

		image, err := io.ReadAll(file)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, errorBody("could not read the uploaded image"))
			return
		}

		draft, err := svc.Scan(r.Context(), userID, image, header.Header.Get("Content-Type"))
		if err != nil {
			writeServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, toDraftResponse(draft))
	}
}
