package awscli

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/cprates/lws/pkg/awsapi"
)

// AwsCli represents an instance of an AWS CLI compatible interface.
type AwsCli struct{}

type dispatchFunc func(
	ctx context.Context,
	reqID string,
	method string,
	path string,
	params map[string]string,
	attributes map[string]string,
	vars map[string]string,
) awsapi.Response

// Install an AWS CLI compatible interface to serve HTTP requests.
func Install(router *mux.Router, region, account, proto, addr string) AwsCli {

	awsCli := AwsCli{}
	awsCli.InstallSQS(router, region, account, proto, addr)
	awsCli.InstallLambda(router, region, account, proto, addr, viper.GetString("lambda.codePath"))

	return awsCli
}

// commonDispatcher returns a function to dispatch requests to the correct endpoints
// based on the Action parameter. This dispatcher will serve requests for LSQS and LSNS.
func commonDispatcher(dispatcherF dispatchFunc) http.HandlerFunc {
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

		ctx := context.WithValue(context.Background(), awsapi.ReqIDKey{}, reqID)
		p, a := flattAndParse(params)
		res := dispatcherF(ctx, reqID, r.Method, r.RequestURI, p, a, mux.Vars(r))
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

// flattAndParse flatten the given map, and parse attributes, converting them into key
// pair elements.
func flattAndParse(rawParams map[string][]string) (params, attrs map[string]string) {

	params = map[string]string{}
	attrs = map[string]string{}

	for k, v := range rawParams {
		if isAttrArray(k) {
			attrs[v[0]] = ""
			continue
		}

		if !isAttrMap("Name", k) {
			if !isAttrMap("Value", k) {
				// regular attributes
				params[k] = v[0]
			}
			continue
		}
		// has pair?
		parts := strings.Split(k, ".")
		attrValKey := "Attribute." + parts[1] + ".Value"
		attrVal, ok := rawParams[attrValKey]
		if !ok {
			continue
		}

		attrs[v[0]] = attrVal[0]
	}

	return
}

func isAttrMap(t, v string) bool {

	if !strings.HasPrefix(v, "Attribute.") {
		return false
	}
	if !strings.HasSuffix(v, "."+t) {
		return false
	}

	parts := strings.Split(v, ".")
	if len(parts) > 3 {
		return false
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return false
	}

	return true
}

func isAttrArray(v string) bool {

	if !strings.HasPrefix(v, "AttributeName.") {
		return false
	}

	parts := strings.Split(v, ".")
	if len(parts) > 2 {
		return false
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return false
	}

	return true
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
