package webprod

import (
	"errors"
	"net/http"

	"github.com/hirokinko/bokiccio/internal/webapp"
)

func NewApplicationHandler(api, ui http.Handler) (http.Handler, error) {
	if api == nil || ui == nil {
		return nil, errors.New("API and UI handlers are required")
	}
	mux := http.NewServeMux()
	mux.Handle("/api", api)
	mux.Handle("/api/", api)
	mux.Handle("/", ui)
	return mux, nil
}

func NewProductionHandler(application http.Handler, validator webapp.IAPTokenValidator, security webapp.IAPSecurity) (http.Handler, error) {
	if application == nil {
		return nil, errors.New("application handler is required")
	}
	private, err := webapp.RequireIAP(application, validator, security)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/livez", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.Header().Set("Allow", "GET, HEAD")
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.WriteHeader(http.StatusOK)
		if request.Method != http.MethodHead {
			_, _ = response.Write([]byte("ok\n"))
		}
	})
	mux.Handle("/", private)
	return mux, nil
}
