package api

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

	"github.com/cprates/lws/common"
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
) common.Result

// InstallAwsCli installs an AWS CLI compatible interface to serve HTTP requests.
func InstallAwsCli(router *mux.Router, region, account, proto, addr string) AwsCli {

	awsCli := AwsCli{}
	awsCli.InstallSQS(router, region, account, proto, addr)
	awsCli.InstallLambda(router, region, account, proto, addr)

	return awsCli
}

// service tries to get the service name from the Authorization info. The service name
// is usually in the URL host name, but that's not true if setting a custom endpoint to be
// used by multiple different services. As a workaround it gets it from the Authorization info
// from the Authorization header, query string or POST body, according to
// https://docs.aws.amazon.com/general/latest/gr/sigv4-signed-request-examples.html#sig-v4-examples-get-query-string
// TODO: should find a better way. DNS doesn't seem the way to go though (affects entire machine)
func service(params url.Values, headers http.Header) string {

	var auth string
	if a, ok := headers["Authorization"]; ok {
		auth = a[0]
	} else if a, ok := params["Authorization"]; ok {
		auth = a[0]
	}

	var service string
	parts := strings.Split(auth, ",")
	for _, p := range parts {
		if s := strings.Split(p, "/"); len(s) == 5 {
			service = s[3]
		}
	}

	return service
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

		service := service(params, r.Header)
		if service == "" {
			log.Errorln("Authorization info not present or invalid,", reqID)
			onAccessDenied(w, viper.GetString("service.protocol")+"://"+r.Host, reqID)
			return
		}

		ctx := context.WithValue(context.Background(), common.ReqIDKey{}, reqID)
		p, a := flattAndParse(params)
		res := dispatcherF(ctx, reqID, r.Method, r.RequestURI, p, a)
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
