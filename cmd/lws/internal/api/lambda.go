package api

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/cprates/lws/common"
	"github.com/cprates/lws/pkg/llambda"
)

var lambdaAction = map[string]reflect.Value{}

// InstallLambda installs Lambda service and starts a new instance of LLambda.
func (a AwsCli) InstallLambda(router *mux.Router, region, accountID, scheme, host string) {

	log.Println("Installing Lambda service")

	root := "/2015-03-31/functions"

	api := llambda.New(region, accountID, scheme, host)
	router.HandleFunc(root, createFunction(api)).Methods(http.MethodPost)
}

func createFunction(api *llambda.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := uuid.NewRandom()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Errorln("Unexpected error", err)
			return
		}
		reqID := u.String()

		log.Debugf("Req %s %q, %s", r.Method, r.RequestURI, reqID)

		w.Header().Add("Content-Type", "application/xml")

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Errorln("Failed to read body,", reqID, err)
			return
		}

		var params url.Values
		switch r.Method {
		case http.MethodPost:
			b := string(body)
			log.Debugf("Req %s, body: %q", reqID, b)
			params, err = url.ParseQuery(b)
		case http.MethodGet:
			params = r.URL.Query()
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Errorln("Failed to read query params,", reqID, err)
			return
		}

		ctx := context.WithValue(context.Background(), common.ReqIDKey{}, reqID)
		p, a := flattAndParse(params)
		res := api.CreateFunction(ctx, p, a)
		if res.Status != 200 {
			log.Debugln("Failed serving req", reqID, res.Err)
			onLwsErr(w, res)
			return
		}

		w.WriteHeader(200)
		_, err = w.Write(res.Result)
		if err != nil {
			log.Errorln("Unexpected error, request", reqID, err)
		}
	}
}
